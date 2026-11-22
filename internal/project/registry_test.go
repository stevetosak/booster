package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stevetosak/booster/internal/gittest"
)

func newRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry(filepath.Join(t.TempDir(), "booster", "projects.json"))
	r.now = func() time.Time { return time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC) }
	return r
}

func TestEmptyRegistry(t *testing.T) {
	o, err := newRegistry(t).Overview()
	if err != nil || len(o.Projects) != 0 || o.Import != nil {
		t.Fatalf("Overview of no file: %+v, %v", o, err)
	}
}

func TestAddResolvesTheMainCheckout(t *testing.T) {
	gittest.Isolate(t)
	root := gittest.Dir(t)
	repo := gittest.Repo(t, filepath.Join(root, "App"))
	wt := gittest.Worktree(t, repo, filepath.Join(root, "app-feature"), "feature")
	r := newRegistry(t)

	p, err := r.Add(wt, "")
	if err != nil || p != (Project{Name: "App", Dir: repo}) {
		t.Fatalf("Add from a worktree: %+v, %v", p, err)
	}
	d, err := r.Get("app")
	if err != nil || d.Name != "App" || len(d.Worktrees) != 2 || d.Worktrees[1].Dir != wt {
		t.Fatalf("Get ignoring case: %+v, %v", d, err)
	}
	info, err := os.Stat(r.path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("projects.json: %v, %v", info, err)
	}
}

func TestAddRefuses(t *testing.T) {
	gittest.Isolate(t)
	root := gittest.Dir(t)
	repo := gittest.Repo(t, filepath.Join(root, "app"))
	other := gittest.Repo(t, filepath.Join(root, "other", "APP"))
	r := newRegistry(t)
	if _, err := r.Add(repo, ""); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, dir, as string
		want          error
	}{
		{"the same checkout again", repo, "again", ErrConflict},
		{"a name in use, other case", other, "", ErrConflict},
		{"a relative path", "app", "", ErrInvalid},
		{"a bad name", other, "a/b", ErrInvalid},
		{"not a repository", gittest.Dir(t), "", ErrNotRepo},
	}
	for _, c := range cases {
		if _, err := r.Add(c.dir, c.as); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if p, err := r.Add(other, "app-other"); err != nil || p.Name != "app-other" {
		t.Errorf("Add with its own name: %+v, %v", p, err)
	}
}

func TestRemove(t *testing.T) {
	gittest.Isolate(t)
	repo := gittest.Repo(t, filepath.Join(gittest.Dir(t), "app"))
	r := newRegistry(t)
	if _, err := r.Add(repo, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove("APP"); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove("app"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Remove: %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Fatalf("the checkout must stay: %v", err)
	}
}

func TestRegistryReadsTheFileOnEachCall(t *testing.T) {
	gittest.Isolate(t)
	repo := gittest.Repo(t, filepath.Join(gittest.Dir(t), "app"))
	r := newRegistry(t)
	write := func(s string) {
		if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(r.path, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"version": 1, "projects": [{"name": "app", "dir": "` + repo + `"}, {"name": "gone", "dir": "/nowhere/gone"}]}`)
	o, err := r.Overview()
	if err != nil || len(o.Projects) != 2 || o.Projects[0].Worktrees[0].Key != "local/app" {
		t.Fatalf("Overview after an edit by hand: %+v, %v", o, err)
	}
	if gone := o.Projects[1]; gone.Error == "" || len(gone.Worktrees) != 0 {
		t.Errorf("a Project whose folder is gone: %+v", gone)
	}

	write(`{"version": 2, "projects": []}`)
	if _, err := r.Overview(); err == nil || !strings.Contains(err.Error(), "version 2") {
		t.Fatalf("a newer version: %v", err)
	}
}
