// Package instance names the running booster instance and its folders. The
// daily booster, built by `make install`, is the main instance; every other
// build is the dev instance, so a booster that an agent builds while it works
// on booster never reaches the Operator's daily core. BOOSTER_INSTANCE names
// another instance for one run. Each instance has its own runtime, state and
// config folders.
package instance

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const (
	Main = "main"
	Dev  = "dev"
	// EnvName names an instance for one run, before the build's own.
	EnvName = "BOOSTER_INSTANCE"
)

// stamped is the instance a build is made for; `make install` sets it to
// main with -ldflags -X.
var stamped string

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Instance is one booster with its own core and folders.
type Instance struct {
	Name string
}

// Current is the instance that BOOSTER_INSTANCE names, else the build's own.
func Current() (Instance, error) {
	return named(cmp.Or(os.Getenv(EnvName), stamped, Dev))
}

func named(name string) (Instance, error) {
	if !validName.MatchString(name) {
		return Instance{}, fmt.Errorf("the instance name %q is not lowercase letters, digits and '-'", name)
	}
	return Instance{Name: name}, nil
}

// folder is the instance's folder in each XDG folder: booster for the main
// instance, booster-<name> for another.
func (i Instance) folder() string {
	if i.Name == Main {
		return "booster"
	}
	return "booster-" + i.Name
}

// RuntimeDir holds the core's socket, lock, core.json and core.log: in
// $XDG_RUNTIME_DIR, which only the user can enter, else a per-user folder in
// the temp folder.
func (i Instance) RuntimeDir() string {
	if base := os.Getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(base) {
		return filepath.Join(base, i.folder())
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", i.folder(), os.Getuid()))
}

// StateDir holds what outlives a reboot but is not configuration (the
// session list): $XDG_STATE_HOME, else ~/.local/state.
func (i Instance) StateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, i.folder()), nil
}

// ConfigDir holds the Operator's configuration (the Project list):
// $XDG_CONFIG_HOME, else ~/.config.
func (i Instance) ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, i.folder()), nil
}
