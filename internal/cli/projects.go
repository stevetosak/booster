package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/stevetosak/booster/internal/project"
)

func projectCommands() []command {
	return []command{
		{"list", "[-json]", "list the Projects, each worktree with its branch and kalfa project key", runProjectList},
		{"add", "[-name N] [folder]", "add the repository around a folder (the current one) by its main checkout", runProjectAdd},
		{"remove", "<name>", "take a Project off the list; its files stay", runProjectRemove},
		{"import", "[-dir D]", "add each repository in D (~/Projects/Active) once", runProjectImport},
	}
}

func runProject(e env, args []string) error {
	if len(args) == 0 {
		usage(os.Stderr, "booster project", projectCommands())
		return ErrUsage
	}
	return dispatch("booster project", projectCommands(), e, args[0], args[1:])
}

func runProjectList(e env, args []string) error {
	flags := flag.NewFlagSet("project list", flag.ContinueOnError)
	asJSON := flags.Bool("json", false, "print the core's answer as JSON")
	if err := parse(flags, args); err != nil {
		return err
	}
	o, err := e.client().Projects()
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(o)
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tBRANCH\tKALFA KEY\tFOLDER")
	for _, p := range o.Projects {
		if p.Error != "" {
			fmt.Fprintf(tw, "%s\t?\t\t%s (%s)\n", p.Name, p.Dir, p.Error)
			continue
		}
		name := p.Name
		for _, w := range p.Worktrees {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", name, branchOf(w), w.Key, w.Dir)
			name = ""
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if o.Import == nil {
		fmt.Println("\nNo import has run: booster project import adds each repository in ~/Projects/Active.")
	}
	return nil
}

// branchOf says what a worktree has checked out, and its state.
func branchOf(w project.Worktree) string {
	b := w.Branch
	switch {
	case w.Bare:
		b = "(bare)"
	case w.Detached:
		b = "(detached)"
	}
	if w.Locked {
		b += " locked"
	}
	if w.Prunable {
		b += " prunable"
	}
	return b
}

func runProjectAdd(e env, args []string) error {
	flags := flag.NewFlagSet("project add", flag.ContinueOnError)
	name := flags.String("name", "", "the Project's name (the main checkout's folder name)")
	if err := parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return usageError("usage: booster project add [-name N] [folder]")
	}
	dir, err := filepath.Abs(flags.Arg(0)) // "" is the current folder
	if err != nil {
		return err
	}
	p, err := e.client().AddProject(dir, *name)
	if err != nil {
		return err
	}
	fmt.Printf("added %s  %s\n", p.Name, p.Dir)
	return nil
}

func runProjectRemove(e env, args []string) error {
	if len(args) != 1 {
		return usageError("usage: booster project remove <name>")
	}
	return e.client().RemoveProject(args[0])
}

func runProjectImport(e env, args []string) error {
	flags := flag.NewFlagSet("project import", flag.ContinueOnError)
	dir := flags.String("dir", filepath.Join(homeDir(), "Projects", "Active"), "the folder of checkouts")
	if err := parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("usage: booster project import [-dir D]")
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	rep, err := e.client().ImportProjects(abs)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, p := range rep.Added {
		fmt.Fprintf(tw, "added\t%s\t%s\n", p.Dir, p.Name)
	}
	for _, f := range rep.Folded {
		fmt.Fprintf(tw, "folded\t%s\tinto %s\n", f.Dir, f.Project)
	}
	for _, s := range rep.Skipped {
		fmt.Fprintf(tw, "skipped\t%s\t%s\n", s.Dir, s.Reason)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Printf("Import of %s at %s: %d added, %d folded, %d skipped.\n",
		rep.Dir, rep.At.Local().Format("2006-01-02 15:04"), len(rep.Added), len(rep.Folded), len(rep.Skipped))
	return nil
}
