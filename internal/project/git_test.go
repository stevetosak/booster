package project

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stevetosak/booster/internal/gittest"
)

func TestParseWorktrees(t *testing.T) {
	out := `worktree /src/app
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /src/app wt
HEAD 2222222222222222222222222222222222222222
detached
locked moving disks

worktree /src/gone
HEAD 3333333333333333333333333333333333333333
branch refs/heads/feat/x
prunable gitdir file points to non-existent location

`
	want := []Worktree{
		{Dir: "/src/app", Head: "1111111111111111111111111111111111111111", Branch: "main"},
		{Dir: "/src/app wt", Head: "2222222222222222222222222222222222222222", Detached: true, Locked: true},
		{Dir: "/src/gone", Head: "3333333333333333333333333333333333333333", Branch: "feat/x", Prunable: true},
	}
	if got := parseWorktrees(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWorktrees:\n got %+v\nwant %+v", got, want)
	}
	if got := parseWorktrees("worktree /srv/app.git\nbare\n"); len(got) != 1 || !got[0].Bare {
		t.Fatalf("bare: %+v", got)
	}
}

func TestWorktreesOfARepository(t *testing.T) {
	gittest.Isolate(t)
	root := gittest.Dir(t)
	repo := gittest.Repo(t, filepath.Join(root, "app"))
	wt := gittest.Worktree(t, repo, filepath.Join(root, "app-feature"), "feature")
	sub := filepath.Join(wt, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	list, err := Worktrees(sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Dir != repo || list[0].Branch != "main" || list[1].Dir != wt || list[1].Branch != "feature" {
		t.Fatalf("Worktrees: %+v", list)
	}
	if list[0].Key != "local/app" || list[1].Key != "local/app-feature" {
		t.Errorf("keys %q and %q, want local/app and local/app-feature (kalfa's root is each worktree's own folder)", list[0].Key, list[1].Key)
	}
	if main, err := MainCheckout(sub); err != nil || main != repo {
		t.Errorf("MainCheckout from a worktree's subfolder: %q, %v", main, err)
	}
}

func TestMainCheckoutRefuses(t *testing.T) {
	gittest.Isolate(t)
	root := gittest.Dir(t)
	repo := gittest.Repo(t, filepath.Join(root, "app"))
	bare := filepath.Join(root, "app.git")
	gittest.Run(t, root, "clone", "-q", "--bare", repo, bare)
	wt := gittest.Worktree(t, bare, filepath.Join(root, "bare-wt"), "feature")

	cases := []struct {
		dir  string
		want error
	}{
		{gittest.Dir(t), ErrNotRepo},
		{bare, ErrNotRepo}, // no .git entry: kalfa sees no project there either
		{wt, ErrBare},
	}
	for _, c := range cases {
		if _, err := MainCheckout(c.dir); !errors.Is(err, c.want) {
			t.Errorf("MainCheckout(%s): %v, want %v", c.dir, err, c.want)
		}
	}
}
