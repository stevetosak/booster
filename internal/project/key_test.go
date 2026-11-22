package project

import (
	"path/filepath"
	"testing"

	"github.com/stevetosak/booster/internal/gittest"
)

// The first five cases are kalfa's own (test/memory.test.ts).
func TestKeyFromRemote(t *testing.T) {
	cases := []struct{ remote, want string }{
		{"git@github.com:Someone/Project.git", "github.com/someone/project"},
		{"https://github.com/someone/project", "github.com/someone/project"},
		{"ssh://git@gitlab.example.org:2222/group/sub/repo.git", "gitlab.example.org/group/sub/repo"},
		{"https://github.com/../etc", ""},
		{"/srv/git/project.git", ""},
		{"github.com:owner/repo.git/", "github.com/owner/repo"},
		{"https://GitHub.com/Owner/Repo.git", "github.com/owner/repo"},
		{"https://github.com/only-one", ""},
		{"https://github.com/owner/re%20po", ""},
		{"git@github.com:/abs/path", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := KeyFromRemote(c.remote); got != c.want {
			t.Errorf("KeyFromRemote(%q) = %q, want %q", c.remote, got, c.want)
		}
	}
}

func TestKeyFromOriginOrFolder(t *testing.T) {
	gittest.Isolate(t)
	root := gittest.Dir(t)
	repo := gittest.Repo(t, filepath.Join(root, "My Repo!"))
	if got := Key(repo); got != "local/my-repo-" {
		t.Errorf("Key with no origin = %q, want local/my-repo-", got)
	}
	gittest.Run(t, repo, "remote", "add", "origin", "git@github.com:Someone/Project.git")
	wt := gittest.Worktree(t, repo, filepath.Join(root, "wt"), "feature")
	for _, dir := range []string{repo, wt} {
		if got := Key(dir); got != "github.com/someone/project" {
			t.Errorf("Key(%s) = %q, want github.com/someone/project", dir, got)
		}
	}
}

func TestKeyFollowsInsteadOf(t *testing.T) {
	gittest.Isolate(t)
	repo := gittest.Repo(t, filepath.Join(gittest.Dir(t), "r"))
	gittest.Run(t, repo, "config", "url.git@github.com:.insteadOf", "gh:")
	gittest.Run(t, repo, "remote", "add", "origin", "gh:someone/project")
	if got := Key(repo); got != "github.com/someone/project" {
		t.Errorf("Key = %q, want the rewritten remote's key", got)
	}
}
