package project

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Worktree is one checkout of a Project's repository, as `git worktree list`
// reports it.
type Worktree struct {
	Dir      string `json:"dir"`
	Head     string `json:"head,omitempty"`
	Branch   string `json:"branch,omitempty"` // without refs/heads/; empty when detached
	Detached bool   `json:"detached,omitempty"`
	Bare     bool   `json:"bare,omitempty"`
	Locked   bool   `json:"locked,omitempty"`
	Prunable bool   `json:"prunable,omitempty"` // its folder is gone
	Key      string `json:"key,omitempty"`      // the kalfa project key; none for a bare or prunable entry
}

// Worktrees lists the checkouts of the repository around dir, the main
// checkout first, each with its kalfa project key.
func Worktrees(dir string) ([]Worktree, error) {
	list, err := listWorktrees(dir)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if !list[i].Bare && !list[i].Prunable {
			list[i].Key = Key(list[i].Dir)
		}
	}
	return list, nil
}

// MainCheckout returns the main checkout of the repository around dir. It
// refuses a folder outside a repository and a repository with no main
// checkout (a bare one).
func MainCheckout(dir string) (string, error) {
	if repoRoot(dir) == "" {
		return "", fmt.Errorf("%w: %s", ErrNotRepo, dir)
	}
	list, err := listWorktrees(dir)
	if err != nil {
		return "", err
	}
	if list[0].Bare {
		return "", fmt.Errorf("%w: %s", ErrBare, list[0].Dir)
	}
	return list[0].Dir, nil
}

// repoRoot returns the nearest folder at or above dir that holds a .git
// entry (a folder, or a file in a linked worktree), as kalfa's repoRootOf
// finds a project's root; "" when there is none.
func repoRoot(dir string) string {
	for at := filepath.Clean(dir); ; at = filepath.Dir(at) {
		if _, err := os.Stat(filepath.Join(at, ".git")); err == nil {
			return at
		}
		if filepath.Dir(at) == at {
			return ""
		}
	}
}

func listWorktrees(dir string) ([]Worktree, error) {
	out, err := git(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	list := parseWorktrees(out)
	if len(list) == 0 {
		return nil, fmt.Errorf("git worktree list in %s printed no worktree", dir)
	}
	return list, nil
}

// parseWorktrees reads the output of `git worktree list --porcelain`: one
// block of "field value" lines per worktree, each opened by its worktree
// line. Without -z (git 2.36+) a path holding a newline cannot be read.
func parseWorktrees(out string) []Worktree {
	var list []Worktree
	for _, line := range strings.Split(out, "\n") {
		field, value, _ := strings.Cut(line, " ")
		if field == "worktree" {
			list = append(list, Worktree{Dir: value})
			continue
		}
		if len(list) == 0 {
			continue
		}
		w := &list[len(list)-1]
		switch field {
		case "HEAD":
			w.Head = value
		case "branch":
			w.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "detached":
			w.Detached = true
		case "bare":
			w.Bare = true
		case "locked":
			w.Locked = true
		case "prunable":
			w.Prunable = true
		}
	}
	return list
}

// gitTimeout bounds each git command, so a hung one cannot hold a request.
const gitTimeout = 10 * time.Second

// repoEnv are the variables that point git at another repository; the
// core's own environment must not steer the commands it runs in a checkout.
var repoEnv = []string{"GIT_DIR=", "GIT_WORK_TREE=", "GIT_COMMON_DIR=", "GIT_INDEX_FILE="}

// git runs git in dir and returns its stdout.
func git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w: %s", strings.Join(args, " "), dir, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return string(out), nil
}

func gitEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !slices.ContainsFunc(repoEnv, func(prefix string) bool { return strings.HasPrefix(kv, prefix) }) {
			env = append(env, kv)
		}
	}
	return env
}
