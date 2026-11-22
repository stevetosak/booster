// Package core is booster's core: one process that outlives the window, owns
// the session host and serves one HTTP/WebSocket API on two listeners, a Unix
// socket for local tools and loopback TCP with a token for the window's page.
package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/stevetosak/booster/internal/jsonfile"
)

// Paths are the core's files in its instance's runtime folder (mode 0700).
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
	ok, err := jsonfile.Read(p.Info, &info)
	if err == nil && !ok {
		err = fmt.Errorf("no core info at %s", p.Info)
	}
	return info, err
}

func writeInfo(p Paths, info Info) error { return jsonfile.Write(p.Info, info) }
