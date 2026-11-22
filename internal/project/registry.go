// Package project is booster's list of Projects. A Project is a git
// repository opened by name; its main checkout is what the registry keeps,
// and its worktrees and their kalfa project keys come from git on each read.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	ErrNotFound = errors.New("no such Project")
	ErrConflict = errors.New("conflict")
	ErrInvalid  = errors.New("invalid")
	ErrNotRepo  = errors.New("not a git repository")
	ErrBare     = errors.New("a bare repository has no main checkout")
)

// Project is one entry of the registry.
type Project struct {
	Name string `json:"name"` // unique, ignoring case
	Dir  string `json:"dir"`  // the main checkout
}

// Detail is a Project with its worktrees, the main checkout first.
type Detail struct {
	Project
	Worktrees []Worktree `json:"worktrees"`
	Error     string     `json:"error,omitempty"` // why git could not list the worktrees
}

// Import records the last import.
type Import struct {
	Dir string    `json:"dir"`
	At  time.Time `json:"at"`
}

// Overview is every Project and the last import; none means it never ran.
type Overview struct {
	Projects []Detail `json:"projects"`
	Import   *Import  `json:"import,omitempty"`
}

// fileVersion is the version of projects.json this booster reads and writes.
const fileVersion = 1

// registryFile is projects.json.
type registryFile struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
	Import   *Import   `json:"import,omitempty"`
}

// Registry keeps the Projects in one JSON file. It reads the file again on
// each call, so an edit by hand counts at once, and replaces the file whole
// on each change.
type Registry struct {
	path string
	mu   sync.Mutex // one change at a time in this process
	now  func() time.Time
}

func NewRegistry(path string) *Registry {
	return &Registry{path: path, now: time.Now}
}

// DefaultPath is $XDG_CONFIG_HOME/booster/projects.json, else the same in
// ~/.config.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "booster", "projects.json"), nil
}

func (r *Registry) Overview() (Overview, error) {
	f, err := r.load()
	if err != nil {
		return Overview{}, err
	}
	o := Overview{Projects: make([]Detail, 0, len(f.Projects)), Import: f.Import}
	for _, p := range f.Projects {
		o.Projects = append(o.Projects, describe(p))
	}
	return o, nil
}

// Get finds a Project by its name, ignoring case.
func (r *Registry) Get(name string) (Detail, error) {
	f, err := r.load()
	if err != nil {
		return Detail{}, err
	}
	i := f.index(name)
	if i < 0 {
		return Detail{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return describe(f.Projects[i]), nil
}

// Add registers the repository around dir, an absolute path, by its main
// checkout. An empty name takes the main checkout's folder name.
func (r *Registry) Add(dir, name string) (Project, error) {
	if !filepath.IsAbs(dir) {
		return Project{}, fmt.Errorf("%w: %q is not an absolute path", ErrInvalid, dir)
	}
	main, err := MainCheckout(dir)
	if err != nil {
		return Project{}, err
	}
	if name == "" {
		name = filepath.Base(main)
	}
	if err := checkName(name); err != nil {
		return Project{}, err
	}
	p := Project{Name: name, Dir: main}
	err = r.change(func(f *registryFile) error {
		if reason := f.conflict(p); reason != "" {
			return fmt.Errorf("%w: %s", ErrConflict, reason)
		}
		f.Projects = append(f.Projects, p)
		return nil
	})
	return p, err
}

// Remove takes a Project off the list; its files stay.
func (r *Registry) Remove(name string) error {
	return r.change(func(f *registryFile) error {
		i := f.index(name)
		if i < 0 {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		f.Projects = slices.Delete(f.Projects, i, i+1)
		return nil
	})
}

// change applies edit to the file as it is now and saves the result.
func (r *Registry) change(edit func(*registryFile) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, err := r.load()
	if err != nil {
		return err
	}
	if err := edit(&f); err != nil {
		return err
	}
	return r.save(f)
}

func (r *Registry) load() (registryFile, error) {
	f := registryFile{Version: fileVersion}
	b, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("%s: %w", r.path, err)
	}
	if f.Version != fileVersion {
		return f, fmt.Errorf("%s: version %d, this booster reads version %d", r.path, f.Version, fileVersion)
	}
	return f, nil
}

// save writes the file to a temporary file beside it and renames it over the
// old one, so a reader never sees half a file.
func (r *Registry) save(f registryFile) error {
	if f.Projects == nil {
		f.Projects = []Project{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(r.path), ".projects-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails once the rename has moved it
	_, err = tmp.Write(append(b, '\n'))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), r.path)
}

// index finds a Project by name, ignoring case; -1 when there is none.
func (f *registryFile) index(name string) int {
	return slices.IndexFunc(f.Projects, func(p Project) bool { return strings.EqualFold(p.Name, name) })
}

// conflict says why p cannot join the list, or "" when it can.
func (f *registryFile) conflict(p Project) string {
	for _, q := range f.Projects {
		if q.Dir == p.Dir {
			return fmt.Sprintf("%s is Project %s", p.Dir, q.Name)
		}
		if strings.EqualFold(q.Name, p.Name) {
			return fmt.Sprintf("the name %s is in use by %s", p.Name, q.Dir)
		}
	}
	return ""
}

func checkName(name string) error {
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.ContainsFunc(name, unicode.IsControl) {
		return fmt.Errorf("%w: %q is not a Project name", ErrInvalid, name)
	}
	return nil
}

func describe(p Project) Detail {
	d := Detail{Project: p, Worktrees: []Worktree{}}
	list, err := Worktrees(p.Dir)
	if err != nil {
		d.Error = err.Error()
		return d
	}
	d.Worktrees = list
	return d
}
