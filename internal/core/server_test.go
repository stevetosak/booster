package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stevetosak/booster/internal/session"
	"github.com/stevetosak/booster/internal/session/ptyhost"
)

const testOrigin = "wails://localhost"

func startCore(t *testing.T) (Paths, *Server, Info) {
	t.Helper()
	dir, err := os.MkdirTemp("", "core") // short: a Unix socket path has a 108-byte limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	p := PathsIn(dir)
	host := ptyhost.New(ptyhost.Options{
		Programs: map[session.Kind][]string{"echo": {"bash", "-c", `printf 'ready\n'; while read -r l; do printf 'echo:%s\n' "$l"; done`}},
		Env:      os.Environ(),
	})
	srv, err := Listen(p, host, testOrigin)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(srv.Stop)
	info, err := ReadInfo(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, srv, info
}

func TestClientWithoutCore(t *testing.T) {
	if _, err := NewClient(PathsIn(t.TempDir())).List(); !errors.Is(err, ErrNoCore) {
		t.Fatalf("List with no core: %v, want ErrNoCore", err)
	}
}

func TestSecondCoreRefused(t *testing.T) {
	p, _, _ := startCore(t)
	if _, err := Listen(p, ptyhost.New(ptyhost.Options{}), testOrigin); err != ErrRunning {
		t.Fatalf("second Listen: %v, want ErrRunning", err)
	}
}

func TestTCPNeedsTokenAndOrigin(t *testing.T) {
	_, _, info := startCore(t)
	cases := []struct {
		name, token, origin string
		want                int
	}{
		{"no token", "", "", http.StatusForbidden},
		{"wrong token", "x", "", http.StatusForbidden},
		{"token, foreign origin", info.Token, "http://localhost", http.StatusForbidden},
		{"token, page origin", info.Token, testOrigin, http.StatusOK},
		{"token, no origin", info.Token, "", http.StatusOK},
	}
	for _, c := range cases {
		req, _ := http.NewRequest(http.MethodGet, "http://"+info.Addr+"/api/sessions", nil)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s: status %d, want %d", c.name, resp.StatusCode, c.want)
		}
	}
}

func TestPreflightAllowsPageOrigin(t *testing.T) {
	_, _, info := startCore(t)
	req, _ := http.NewRequest(http.MethodOptions, "http://"+info.Addr+"/api/sessions", nil)
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Access-Control-Allow-Origin") != testOrigin {
		t.Fatalf("preflight: %d, allow-origin %q", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

// TestAttachOverTCP creates a session over the Unix socket, attaches over TCP
// as the page does, and sends input and a resize.
func TestAttachOverTCP(t *testing.T) {
	p, _, info := startCore(t)
	c := NewClient(p)
	s, err := c.Create(session.Spec{Kind: "echo", Dir: t.TempDir(), Size: session.Size{Cols: 80, Rows: 24}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var ws *websocket.Conn
	var snapshot []byte
	for !strings.Contains(string(snapshot), "ready") { // attach until the program has printed
		if ws != nil {
			ws.CloseNow()
			time.Sleep(20 * time.Millisecond)
		}
		ws, _, err = websocket.Dial(ctx, "ws://"+info.Addr+"/api/sessions/"+s.ID+"/attach?token="+info.Token,
			&websocket.DialOptions{HTTPHeader: http.Header{"Origin": {testOrigin}}})
		if err != nil {
			t.Fatal(err)
		}
		typ, hello, err := ws.Read(ctx)
		if err != nil || typ != websocket.MessageText || !strings.Contains(string(hello), `"cols":80`) {
			t.Fatalf("hello: %v %s %v", typ, hello, err)
		}
		if _, snapshot, err = ws.Read(ctx); err != nil {
			t.Fatal(err)
		}
	}
	defer ws.CloseNow()

	if err := ws.Write(ctx, websocket.MessageBinary, []byte("hi\r")); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for !strings.Contains(out.String(), "echo:hi") {
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v (got %q)", err, out.String())
		}
		out.Write(b)
	}

	resize, _ := json.Marshal(map[string]any{"type": "resize", "cols": 100, "rows": 30})
	if err := ws.Write(ctx, websocket.MessageText, resize); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		list, err := c.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) == 1 && list[0].Size == (session.Size{Cols: 100, Rows: 30}) && list[0].Clients == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after the resize: %+v", list)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
