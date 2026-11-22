package instance

import (
	"path/filepath"
	"testing"
)

func TestCurrent(t *testing.T) {
	cases := []struct {
		env, stamp, want string
	}{
		{"", "", Dev},
		{"", Main, Main},
		{"try-2", Main, "try-2"},
	}
	for _, c := range cases {
		t.Setenv(EnvName, c.env)
		stamped = c.stamp
		i, err := Current()
		if err != nil || i.Name != c.want {
			t.Errorf("env %q, stamp %q: %+v, %v; want %s", c.env, c.stamp, i, err, c.want)
		}
	}
	stamped = ""
	t.Setenv(EnvName, "../x")
	if _, err := Current(); err == nil {
		t.Error("a name with a path: no error")
	}
}

func TestFolders(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	t.Setenv("XDG_STATE_HOME", "/s")
	t.Setenv("XDG_CONFIG_HOME", "/c")
	for _, c := range []struct{ name, folder string }{{Main, "booster"}, {Dev, "booster-dev"}} {
		i := Instance{Name: c.name}
		state, _ := i.StateDir()
		config, _ := i.ConfigDir()
		if i.RuntimeDir() != filepath.Join("/run/user/1000", c.folder) || state != filepath.Join("/s", c.folder) || config != filepath.Join("/c", c.folder) {
			t.Errorf("%s: %s, %s, %s", c.name, i.RuntimeDir(), state, config)
		}
	}
	t.Setenv("XDG_STATE_HOME", "relative")
	t.Setenv("HOME", "/home/op")
	if state, _ := (Instance{Name: Main}).StateDir(); state != "/home/op/.local/state/booster" {
		t.Errorf("a relative XDG_STATE_HOME: %s", state)
	}
}
