package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"booster/spikes/session/session"
)

// Client talks to the core over its Unix socket.
type Client struct {
	http *http.Client
}

func NewClient(p Paths) *Client {
	return &Client{http: &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", p.Socket)
		}},
	}}
}

// Ping reports whether a core answers.
func (c *Client) Ping() error { return c.do(http.MethodGet, "/api/ping", nil, nil) }

func (c *Client) List() ([]session.Info, error) {
	var out []session.Info
	return out, c.do(http.MethodGet, "/api/sessions", nil, &out)
}

func (c *Client) Create(spec session.Spec) (session.Info, error) {
	var out session.Info
	return out, c.do(http.MethodPost, "/api/sessions", spec, &out)
}

func (c *Client) Prompt(id, text string, submit bool) error {
	return c.do(http.MethodPost, "/api/sessions/"+id+"/prompt", promptRequest{Text: text, Submit: submit}, nil)
}

func (c *Client) Close(id string) error { return c.do(http.MethodDelete, "/api/sessions/"+id, nil, nil) }

func (c *Client) Shutdown() error { return c.do(http.MethodPost, "/api/shutdown", nil, nil) }

func (c *Client) do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://core"+path, body)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: %s", method, path, bytes.TrimSpace(msg))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Ensure returns the running core's Info, and first starts a detached core
// (`self core`, in its own session, with env) when none answers.
func Ensure(p Paths, self string, env []string) (Info, error) {
	c := NewClient(p)
	if c.Ping() == nil {
		return ReadInfo(p)
	}
	if err := p.ensureDir(); err != nil {
		return Info{}, err
	}
	logf, err := os.OpenFile(p.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return Info{}, err
	}
	defer logf.Close()
	cmd := exec.Command(self, "core")
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // the core outlives the window and its terminal
	if err := cmd.Start(); err != nil {
		return Info{}, fmt.Errorf("start the core: %w", err)
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.Ping() == nil {
			return ReadInfo(p)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return Info{}, errors.New("the core did not answer within 5 s; see " + p.Log)
}
