// Package core is booster's core: one process that outlives the window, owns
// the session host and serves one HTTP/WebSocket API on two listeners, a Unix
// socket for local tools and loopback TCP with a token for the window's page.
package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Paths are the core's files in the user's runtime folder (mode 0700).
type Paths struct {
	Dir    string
	Socket string // the Unix listener (mode 0600)
	Info   string // Info as JSON (mode 0600)
	Lock   string // held while a core runs
	Log    string // the detached core's stdout and stderr
}

// Info tells the window how to reach the core's TCP listener.
type Info struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

// runtimeName names the core's folder in the runtime folder.
const runtimeName = "booster"

// DefaultPaths uses $XDG_RUNTIME_DIR, which only the user can enter, else a
// per-user folder in the temp folder.
func DefaultPaths() Paths {
	base, name := os.Getenv("XDG_RUNTIME_DIR"), runtimeName
	if base == "" {
		base, name = os.TempDir(), fmt.Sprintf("%s-%d", runtimeName, os.Getuid())
	}
	return PathsIn(filepath.Join(base, name))
}

// PathsIn lays the core's files out in dir.
func PathsIn(dir string) Paths {
	return Paths{
		Dir:    dir,
		Socket: filepath.Join(dir, "core.sock"),
		Info:   filepath.Join(dir, "core.json"),
		Lock:   filepath.Join(dir, "core.lock"),
		Log:    filepath.Join(dir, "core.log"),
	}
}

func (p Paths) ensureDir() error {
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(p.Dir, 0o700)
}

// ReadInfo reads the running core's Info.
func ReadInfo(p Paths) (Info, error) {
	var info Info
	b, err := os.ReadFile(p.Info)
	if err != nil {
		return info, err
	}
	return info, json.Unmarshal(b, &info)
}

func writeInfo(p Paths, info Info) error {
	b, err := json.Marshal(info)
	if err != nil {
		return err
	}
	tmp := p.Info + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.Info)
}
