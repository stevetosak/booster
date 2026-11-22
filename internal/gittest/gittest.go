// Package gittest makes git repositories for tests, with git cut off from the
// user's and the system's configuration.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Isolate points git at no global or system configuration and gives it an
// author, for the rest of the test. A test that calls it cannot run in
// parallel.
func Isolate(t testing.TB) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, who := range []string{"GIT_AUTHOR", "GIT_COMMITTER"} {
		t.Setenv(who+"_NAME", "booster test")
		t.Setenv(who+"_EMAIL", "test@booster.invalid")
	}
}

// Dir is a new empty folder by its real path, as git names it.
func Dir(t testing.TB) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// Repo makes a repository at dir with one empty commit on main.
func Repo(t testing.TB, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	Run(t, dir, "init", "-q", "-b", "main")
	Run(t, dir, "commit", "-q", "--allow-empty", "-m", "first")
	return dir
}

// Worktree adds a linked worktree of repo at dir, on a new branch.
func Worktree(t testing.TB, repo, dir, branch string) string {
	t.Helper()
	Run(t, repo, "worktree", "add", "-q", "-b", branch, dir)
	return dir
}

// Run runs git in dir and returns its stdout; a failure ends the test.
func Run(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
