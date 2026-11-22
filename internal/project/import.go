package project

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ImportReport says what an import did with each folder.
type ImportReport struct {
	Dir     string    `json:"dir"`
	At      time.Time `json:"at"`
	Added   []Project `json:"added"`
	Folded  []Folded  `json:"folded"`
	Skipped []Skipped `json:"skipped"`
}

// Folded is a linked worktree the import found, under its Project.
type Folded struct {
	Dir     string `json:"dir"`
	Project string `json:"project"`
}

// Skipped is a folder the import did not register, and why.
type Skipped struct {
	Dir    string `json:"dir"`
	Reason string `json:"reason"`
}

// Import registers each repository in dir, an absolute path, once, by its
// main checkout, named by that checkout's folder. A linked worktree folds
// into its main checkout's Project, which the import registers when the list
// does not have it yet. The registry records that the import ran.
func (r *Registry) Import(dir string) (ImportReport, error) {
	if !filepath.IsAbs(dir) {
		return ImportReport{}, fmt.Errorf("%w: %q is not an absolute path", ErrInvalid, dir)
	}
	rep := ImportReport{Dir: dir, Added: []Project{}, Folded: []Folded{}, Skipped: []Skipped{}}
	mains, worktrees, err := scan(dir, &rep)
	if err != nil {
		return ImportReport{}, err
	}
	err = r.change(func(f *registryFile) error {
		tried := map[string]bool{} // main checkouts the import tried to register
		for _, c := range mains {
			tried[c.main] = true
			if i := f.indexDir(c.main); i >= 0 {
				rep.Skipped = append(rep.Skipped, Skipped{c.dir, "already Project " + f.Projects[i].Name})
				continue
			}
			rep.register(f, c.main)
		}
		for _, c := range worktrees {
			if !tried[c.main] && f.indexDir(c.main) < 0 {
				rep.register(f, c.main) // a main checkout outside dir
			}
			tried[c.main] = true
			if i := f.indexDir(c.main); i >= 0 {
				rep.Folded = append(rep.Folded, Folded{c.dir, f.Projects[i].Name})
			} else {
				rep.Skipped = append(rep.Skipped, Skipped{c.dir, "its main checkout " + c.main + " is not a Project"})
			}
		}
		rep.At = r.now().UTC().Truncate(time.Second)
		f.Import = &Import{Dir: dir, At: rep.At}
		return nil
	})
	return rep, err
}

// checkout is a folder the import found in a repository: its own folder and
// its repository's main checkout.
type checkout struct{ dir, main string }

// scan sorts the folders in dir into main checkouts and linked worktrees; it
// reports each other folder as skipped. It runs git, so it runs outside the
// registry's lock.
func scan(dir string, rep *ImportReport) (mains, worktrees []checkout, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			continue // a Project is a folder; a plain file is not reported
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			rep.Skipped = append(rep.Skipped, Skipped{path, ErrNotRepo.Error()})
			continue
		}
		main, err := MainCheckout(path)
		if err != nil {
			rep.Skipped = append(rep.Skipped, Skipped{path, err.Error()})
			continue
		}
		// git names the main checkout by its real path; a folder reached
		// through a symbolic link is that checkout too.
		if real, err := filepath.EvalSymlinks(path); err == nil && real == main {
			mains = append(mains, checkout{path, main})
		} else {
			worktrees = append(worktrees, checkout{path, main})
		}
	}
	return mains, worktrees, nil
}

// register adds the main checkout at dir under its folder's name, or reports
// why it cannot.
func (rep *ImportReport) register(f *registryFile, dir string) bool {
	p := Project{Name: filepath.Base(dir), Dir: dir}
	if reason := f.conflict(p); reason != "" {
		rep.Skipped = append(rep.Skipped, Skipped{dir, reason})
		return false
	}
	f.Projects = append(f.Projects, p)
	rep.Added = append(rep.Added, p)
	return true
}

func (f *registryFile) indexDir(dir string) int {
	for i, p := range f.Projects {
		if p.Dir == dir {
			return i
		}
	}
	return -1
}
