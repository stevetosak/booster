package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stevetosak/booster/internal/gittest"
)

// TestProjectsOverSocket drives the Project API as the CLI does.
func TestProjectsOverSocket(t *testing.T) {
	gittest.Isolate(t)
	active := gittest.Dir(t)
	app := gittest.Repo(t, filepath.Join(active, "app"))
	gittest.Worktree(t, app, filepath.Join(active, "app-feature"), "feature")
	p, _, _ := startCore(t)
	c := NewClient(p)

	if _, err := c.AddProject("app", ""); err == nil || !strings.Contains(err.Error(), "not an absolute path") {
		t.Fatalf("AddProject with a relative path: %v", err)
	}
	rep, err := c.ImportProjects(active)
	if err != nil || len(rep.Added) != 1 || len(rep.Folded) != 1 {
		t.Fatalf("ImportProjects: %+v, %v", rep, err)
	}
	if _, err := c.AddProject(app, ""); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("AddProject twice: %v", err)
	}
	o, err := c.Projects()
	if err != nil || len(o.Projects) != 1 || len(o.Projects[0].Worktrees) != 2 || o.Import == nil {
		t.Fatalf("Projects: %+v, %v", o, err)
	}
	if d, err := c.Project("APP"); err != nil || d.Dir != app || d.Worktrees[1].Key != "local/app-feature" {
		t.Fatalf("Project: %+v, %v", d, err)
	}
	if err := c.RemoveProject("app"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Project("app"); err == nil || !strings.Contains(err.Error(), "no such Project") {
		t.Fatalf("Project after RemoveProject: %v", err)
	}
}
