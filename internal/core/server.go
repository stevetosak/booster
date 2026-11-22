package core

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/stevetosak/booster/internal/project"
	"github.com/stevetosak/booster/internal/session"
)

// ErrRunning means another core holds the lock.
var ErrRunning = errors.New("a core is already running")

// Config is what the core serves.
type Config struct {
	Paths    Paths
	Backend  session.Backend
	Projects *project.Registry
	// PageOrigin is the whole origin (scheme included) of the window's page,
	// the only origin let in over TCP.
	PageOrigin string
}

// Server serves the API on both listeners.
type Server struct {
	backend  session.Backend
	projects *project.Registry
	origin   string
	paths    Paths
	token    string

	lock     *os.File
	uds, tcp net.Listener
	http     *http.Server
	stopOnce sync.Once
	stopped  chan struct{}
}

type unixKey struct{}

// Listen takes the core's lock and opens both listeners.
func Listen(cfg Config) (_ *Server, err error) {
	p := cfg.Paths
	if err := p.ensureDir(); err != nil {
		return nil, err
	}
	// Each step that holds a resource adds its release here; they run in
	// reverse when a later step fails.
	var undo []func()
	defer func() {
		if err != nil {
			for i := len(undo) - 1; i >= 0; i-- {
				undo[i]()
			}
		}
	}()

	lock, err := os.OpenFile(p.Lock, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	undo = append(undo, func() { _ = lock.Close() })
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, ErrRunning
	}
	_ = os.Remove(p.Socket) // left by a core that did not stop cleanly; the lock proves it is gone
	uds, err := net.Listen("unix", p.Socket)
	if err != nil {
		return nil, err
	}
	undo = append(undo, func() { _ = uds.Close() })
	if err := os.Chmod(p.Socket, 0o600); err != nil {
		return nil, err
	}
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	undo = append(undo, func() { _ = tcp.Close() })
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	s := &Server{
		backend: cfg.Backend, projects: cfg.Projects, origin: cfg.PageOrigin, paths: p,
		token: hex.EncodeToString(tok), lock: lock, uds: uds, tcp: tcp, stopped: make(chan struct{}),
	}
	if err := writeInfo(p, Info{Addr: tcp.Addr().String(), Token: s.token, PID: os.Getpid()}); err != nil {
		return nil, err
	}
	s.http = &http.Server{
		Handler: s.routes(),
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			if c.LocalAddr().Network() == "unix" {
				return context.WithValue(ctx, unixKey{}, true)
			}
			return ctx
		},
	}
	return s, nil
}

// Serve runs until Stop.
func (s *Server) Serve() {
	go func() { _ = s.http.Serve(s.uds) }()
	go func() { _ = s.http.Serve(s.tcp) }()
	<-s.stopped
}

// Stop ends every session, closes the listeners and removes the core's files.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		s.backend.Shutdown()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.http.Shutdown(ctx)
		_ = os.Remove(s.paths.Socket)
		_ = os.Remove(s.paths.Info)
		_ = s.lock.Close()
		close(s.stopped)
	})
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]int{"pid": os.Getpid()}) })
	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.backend.List()) })
	mux.HandleFunc("POST /api/sessions", s.handleCreate)
	mux.HandleFunc("DELETE /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.reply(w, s.backend.Close(r.PathValue("id")), nil)
	})
	mux.HandleFunc("POST /api/sessions/{id}/prompt", s.handlePrompt)
	mux.HandleFunc("GET /api/sessions/{id}/attach", s.handleAttach)
	mux.HandleFunc("POST /api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"stopping": true})
		go s.Stop()
	})
	s.projectRoutes(mux)
	return s.guard(mux)
}

// guard lets in every request on the Unix socket (its mode 0600 limits it to
// the user) and, over TCP, only requests with the token whose Origin, when
// the client sends one, is the window page's. A WebSocket from the page
// cannot set headers, so the token may also come as ?token=.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(unixKey{}) != nil {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != s.origin {
			log.Printf("refused %s %s from origin %q", r.Method, r.URL.Path, origin)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if origin != "" {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", s.origin)
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
			h.Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent) // a CORS preflight carries no token
			return
		}
		given := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if given == "" {
			given = r.URL.Query().Get("token")
		}
		if subtle.ConstantTimeCompare([]byte(given), []byte(s.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var spec session.Spec
	if !decode(w, r, &spec) {
		return
	}
	info, err := s.backend.Create(spec)
	s.reply(w, err, info)
}

type promptRequest struct {
	Text   string `json:"text"`
	Submit bool   `json:"submit"`
}

func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	var req promptRequest
	if !decode(w, r, &req) {
		return
	}
	s.reply(w, s.backend.Prompt(r.PathValue("id"), req.Text, req.Submit), nil)
}

func (s *Server) reply(w http.ResponseWriter, err error, v any) {
	switch {
	case errors.Is(err, session.ErrNoSession), errors.Is(err, project.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, session.ErrUnknownKind), errors.Is(err, project.ErrInvalid):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, project.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case err != nil:
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case v == nil:
		writeJSON(w, map[string]bool{"ok": true})
	default:
		writeJSON(w, v)
	}
}

// decode reads a JSON request body into v; on a fault it answers 400 and
// returns false.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// The attach protocol. The core sends a text "hello" with the snapshot's
// size, one binary message with the snapshot, then binary output; a text
// "end" says why the output stopped. The client sends binary input and text
// "resize" messages.
type (
	helloMessage struct {
		Type string       `json:"type"` // "hello"
		Size session.Size `json:"size"`
	}
	endMessage struct {
		Type   string `json:"type"`   // "end"
		Reason string `json:"reason"` // "ended" or "too-slow"
	}
	clientMessage struct {
		Type string `json:"type"` // "resize"
		Cols uint16 `json:"cols"`
		Rows uint16 `json:"rows"`
	}
)

func (s *Server) handleAttach(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := s.backend.Attach(id)
	if err != nil {
		s.reply(w, err, nil)
		return
	}
	defer a.Detach()
	log.Printf("session %s: attached from %s (snapshot %d bytes for %dx%d)", id, r.RemoteAddr, len(a.Snapshot()), a.Size().Cols, a.Size().Rows)
	defer log.Printf("session %s: detached from %s", id, r.RemoteAddr)
	// The guard checked the Origin over TCP; the Unix socket has none.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		s.readInput(ctx, conn, id)
	}()

	hello, _ := json.Marshal(helloMessage{Type: "hello", Size: a.Size()})
	if conn.Write(ctx, websocket.MessageText, hello) != nil ||
		conn.Write(ctx, websocket.MessageBinary, a.Snapshot()) != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-a.Output():
			if !ok {
				reason := "ended"
				if errors.Is(a.Err(), session.ErrTooSlow) {
					reason = "too-slow"
				}
				end, _ := json.Marshal(endMessage{Type: "end", Reason: reason})
				_ = conn.Write(ctx, websocket.MessageText, end)
				conn.Close(websocket.StatusNormalClosure, reason)
				return
			}
			if conn.Write(ctx, websocket.MessageBinary, chunk) != nil {
				return
			}
		}
	}
}

func (s *Server) readInput(ctx context.Context, conn *websocket.Conn, id string) {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageBinary {
			if err := s.backend.Write(id, data); err != nil {
				return
			}
			continue
		}
		var m clientMessage
		if json.Unmarshal(data, &m) == nil && m.Type == "resize" {
			if err := s.backend.Resize(id, session.Size{Cols: m.Cols, Rows: m.Rows}); err != nil {
				log.Printf("session %s: resize: %v", id, err)
			}
		}
	}
}

// String names the listeners for the log.
func (s *Server) String() string {
	return fmt.Sprintf("unix %s, tcp %s", s.paths.Socket, s.tcp.Addr())
}
