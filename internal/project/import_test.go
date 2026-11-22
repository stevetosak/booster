package project

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stevetosak/booster/internal/gittest"
)

// TestImport lays out a folder of Projects as ~/Projects/Active is: main
// checkouts, a worktree beside its main checkout, a worktree whose main
// checkout is elsewhere, a folder that is not a repository, a name that is
// taken, and a plain file.
func TestImport(t *testing.T) {
	gittest.Isolate(t)
	active, elsewhere := gittest.Dir(t), gittest.Dir(t)
	app := gittest.Repo(t, filepath.Join(active, "app"))
	appWT := gittest.Worktree(t, app, filepath.Join(active, "app-feature"), "feature")
	lib := gittest.Repo(t, filepath.Join(active, "lib"))
	tool := gittest.Repo(t, filepath.Join(elsewhere, "tool"))
	toolWT := gittest.Worktree(t, tool, filepath.Join(active, "tool-fix"), "fix")
	notes := filepath.Join(active, "notes")
	if err := os.Mkdir(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "README"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := newRegistry(t)
	libElsewhere := gittest.Repo(t, filepath.Join(elsewhere, "lib"))
	if _, err := r.Add(libElsewhere, ""); err != nil {
		t.Fatal(err)
	}

	rep, err := r.Import(active)
	if err != nil {
		t.Fatal(err)
	}
	wantAdded := []Project{{"app", app}, {"tool", tool}}
	wantFolded := []Folded{{appWT, "app"}, {toolWT, "tool"}}
	wantSkipped := []Skipped{
		{lib, "the name lib is in use by " + libElsewhere},
		{notes, "not a git repository"},
	}
	if !reflect.DeepEqual(rep.Added, wantAdded) || !reflect.DeepEqual(rep.Folded, wantFolded) || !reflect.DeepEqual(sortedSkips(rep.Skipped), wantSkipped) {
		t.Fatalf("first import:\n added %+v\nfolded %+v\nskipped %+v", rep.Added, rep.Folded, rep.Skipped)
	}
	o, err := r.Overview()
	if err != nil || len(o.Projects) != 3 || o.Import == nil || o.Import.Dir != active || !o.Import.At.Equal(rep.At) {
		t.Fatalf("after the import: %+v, %v", o, err)
	}

	// A second run registers nothing twice.
	rep, err = r.Import(active)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 0 || !reflect.DeepEqual(rep.Folded, wantFolded) || len(rep.Skipped) != 3 {
		t.Fatalf("second import: %+v", rep)
	}
	if o, _ := r.Overview(); len(o.Projects) != 3 {
		t.Fatalf("after the second import: %d Projects, want 3", len(o.Projects))
	}
}

func TestImportNeedsAnAbsoluteFolder(t *testing.T) {
	if _, err := newRegistry(t).Import("Projects/Active"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Import of a relative path: %v, want ErrInvalid", err)
	}
}

// sortedSkips orders the skipped folders by path: main checkouts and other
// folders are reported in different passes.
func sortedSkips(s []Skipped) []Skipped {
	out := slices.Clone(s)
	slices.SortFunc(out, func(a, b Skipped) int { return strings.Compare(a.Dir, b.Dir) })
	return out
}
