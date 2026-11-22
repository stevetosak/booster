package main

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
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// quitDelay lets the frontend read the reply before the window closes.
const quitDelay = 300 * time.Millisecond

// server gives the frontend its terminals and a control channel over
// WebSockets on loopback. Every connection must carry the random token that
// only the window's URL holds: a WebSocket ignores the same-origin rule, so
// without it any local web page could open a shell.
type server struct {
	cfg      config
	token    string
	self     string
	listener net.Listener
	quit     func()
}

func newServer(cfg config) (*server, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find this binary: %w", err)
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen on loopback: %w", err)
	}
	return &server{cfg: cfg, token: token, self: self, listener: ln, quit: func() {}}, nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("make a token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// frontendQuery is what the page needs to reach the server and run.
func (s *server) frontendQuery() url.Values {
	q := url.Values{}
	q.Set("ws", "ws://"+s.listener.Addr().String())
	q.Set("token", s.token)
	q.Set("renderer", s.cfg.renderer)
	q.Set("label", s.cfg.label)
	if s.cfg.autobench != "" {
		q.Set("autobench", "1")
	}
	if s.cfg.trace {
		q.Set("trace", "1")
	}
	return q
}

func (s *server) serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/pty", s.handlePTY)
	mux.HandleFunc("/control", s.handleControl)
	return http.Serve(s.listener, mux)
}

func (s *server) accept(w http.ResponseWriter, r *http.Request) (*websocket.Conn, bool) {
	given := r.URL.Query().Get("token")
	if subtle.ConstantTimeCompare([]byte(given), []byte(s.token)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, false
	}
	// The whole origin, scheme included, must be the window's: a host pattern
	// alone would also let in an http://localhost page.
	if origin := r.Header.Get("Origin"); origin != webview.Origin {
		log.Printf("refused %s from origin %q: the page origin of this build is %q", r.URL.Path, origin, webview.Origin)
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, false
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // the origin is checked above
	if err != nil {
		log.Printf("refused %s from origin %q: %v", r.URL.Path, r.Header.Get("Origin"), err)
		return nil, false
	}
	conn.SetReadLimit(1 << 20)
	return conn, true
}

// resizeMessage is the one text message a terminal connection takes; input
// arrives as binary messages.
type resizeMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (s *server) handlePTY(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	argv, dir, err := s.command(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cols, rows := parseSize(q)
	conn, ok := s.accept(w, r)
	if !ok {
		return
	}
	defer conn.CloseNow()

	sess, err := startPTY(argv, dir, cols, rows)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "start: "+err.Error())
		return
	}
	defer sess.close()

	ctx := r.Context()
	go func() {
		pumpInput(ctx, conn, sess)
		sess.close() // the page closed the terminal: end the program
	}()
	pumpOutput(ctx, conn, sess)
	conn.Close(websocket.StatusNormalClosure, "exited")
}

// command maps a terminal request to a program; the page never names a
// program itself.
func (s *server) command(q url.Values) (argv []string, dir string, err error) {
	switch q.Get("kind") {
	case "main":
		return []string{"/bin/sh", "-c", "exec " + s.cfg.cmd}, s.cfg.dir, nil
	case "cat":
		return []string{"cat"}, os.TempDir(), nil
	case "gen":
		gen := q.Get("gen")
		if gen != "plain" && gen != "color" {
			return nil, "", fmt.Errorf("gen must be plain or color")
		}
		size, err1 := strconv.ParseInt(q.Get("bytes"), 10, 64)
		rate, err2 := strconv.ParseInt(q.Get("rate"), 10, 64)
		if err := errors.Join(err1, err2); err != nil || size < 0 || rate < 0 {
			return nil, "", fmt.Errorf("bytes and rate must be whole numbers, 0 or more")
		}
		return []string{s.self, generatorCommand, "-kind", gen,
			"-bytes", strconv.FormatInt(size, 10), "-rate", strconv.FormatInt(rate, 10)}, os.TempDir(), nil
	default:
		return nil, "", fmt.Errorf("unknown terminal kind %q", q.Get("kind"))
	}
}

func parseSize(q url.Values) (cols, rows uint16) {
	cols, rows = 80, 24
	if c, err := strconv.ParseUint(q.Get("cols"), 10, 16); err == nil && c > 0 {
		cols = uint16(c)
	}
	if r, err := strconv.ParseUint(q.Get("rows"), 10, 16); err == nil && r > 0 {
		rows = uint16(r)
	}
	return cols, rows
}

func pumpOutput(ctx context.Context, conn *websocket.Conn, sess *ptySession) {
	buf := make([]byte, 32<<10)
	for {
		n, err := sess.Read(buf)
		if n > 0 {
			if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return // EIO: the program exited and its output is drained
		}
	}
}

func pumpInput(ctx context.Context, conn *websocket.Conn, sess *ptySession) {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageBinary {
			if _, err := sess.Write(data); err != nil {
				return
			}
			continue
		}
		var msg resizeMessage
		if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
			_ = sess.resize(msg.Cols, msg.Rows)
		}
	}
}

// controlRequest asks for the memory of the process tree, logs the page's
// progress or terminal trace, or saves a report.
type controlRequest struct {
	Op     string          `json:"op"`
	Text   string          `json:"text,omitempty"`
	Report json.RawMessage `json:"report,omitempty"`
}

type controlReply struct {
	Op     string        `json:"op"`
	Memory *memoryReport `json:"memory,omitempty"`
	Path   string        `json:"path,omitempty"`
	Error  string        `json:"error,omitempty"`
}

func (s *server) handleControl(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.accept(w, r)
	if !ok {
		return
	}
	defer conn.CloseNow()
	ctx := r.Context()
	for {
		var req controlRequest
		if err := wsjson.Read(ctx, conn, &req); err != nil {
			return
		}
		reply := s.control(req)
		if err := wsjson.Write(ctx, conn, reply); err != nil {
			return
		}
		if req.Op == "report" && s.cfg.autobench != "" {
			time.AfterFunc(quitDelay, s.quit)
		}
	}
}

func (s *server) control(req controlRequest) controlReply {
	switch req.Op {
	case "memory":
		mem, err := readMemory()
		if err != nil {
			return controlReply{Op: req.Op, Error: err.Error()}
		}
		return controlReply{Op: req.Op, Memory: &mem}
	case "progress":
		// The log is how a watcher sees a run advance or stall.
		log.Printf("progress: %s", req.Text)
		return controlReply{Op: req.Op}
	case "trace":
		log.Printf("trace: %s", req.Text)
		return controlReply{Op: req.Op}
	case "report":
		path, err := s.saveReport(req.Report)
		if err != nil {
			return controlReply{Op: req.Op, Error: err.Error()}
		}
		return controlReply{Op: req.Op, Path: path}
	default:
		return controlReply{Op: req.Op, Error: "unknown op"}
	}
}

// webviewBuild names the WebKitGTK API a build links, its Debian package, and
// the origin of the window's page.
type webviewBuild struct {
	API     string `json:"api"`
	Package string `json:"package"`
	Version string `json:"version"`
	Origin  string `json:"origin"`
}

// runReport is one run: the page's measurements with what only this side knows.
type runReport struct {
	Label     string            `json:"label"`
	Renderer  string            `json:"renderer"`
	GPUPolicy string            `json:"gpuPolicy"`
	Time      string            `json:"time"`
	Env       map[string]string `json:"env"`
	Webview   webviewBuild      `json:"webview"`
	Memory    *memoryReport     `json:"memory,omitempty"`
	GPU       []processGPU      `json:"gpu,omitempty"`
	Page      json.RawMessage   `json:"page"`
}

// reportedEnv are the variables that change how WebKitGTK and the GPU draw.
var reportedEnv = []string{
	"BOOSTER_WEBVIEW_GPU",
	"WEBKIT_DISABLE_DMABUF_RENDERER", "WEBKIT_DISABLE_COMPOSITING_MODE", "GSK_RENDERER",
	"__NV_PRIME_RENDER_OFFLOAD", "__GLX_VENDOR_LIBRARY_NAME", "__EGL_VENDOR_LIBRARY_FILENAMES", "DISPLAY", "WAYLAND_DISPLAY",
}

func (s *server) saveReport(page json.RawMessage) (string, error) {
	rep := runReport{
		Label:     s.cfg.label,
		Renderer:  s.cfg.renderer,
		GPUPolicy: s.cfg.gpuPolicy,
		Time:      time.Now().Format(time.RFC3339),
		Env:       map[string]string{},
		Webview:   webview,
		Page:      page,
	}
	rep.Webview.Version = packageVersion(webview.Package)
	for _, k := range reportedEnv {
		if v, ok := os.LookupEnv(k); ok {
			rep.Env[k] = v
		}
	}
	if mem, err := readMemory(); err == nil {
		rep.Memory = &mem
		rep.GPU = readGPU(mem.Processes)
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", err
	}
	path := s.cfg.autobench
	if path == "" {
		path = fmt.Sprintf("report-%s.json", time.Now().Format("20060102-150405"))
	}
	return path, os.WriteFile(path, data, 0o644)
}

func packageVersion(pkg string) string {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Version}", pkg).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
