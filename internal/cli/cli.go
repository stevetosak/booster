// Package cli is booster's command line. One binary has every role: the
// window (the default command), the core that owns the agent sessions, and
// the tools that talk to a running core over its Unix socket.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"text/tabwriter"
)

// ErrUsage means the command line was wrong. The fault is already reported on
// stderr, so the caller only exits with status 2.
var ErrUsage = errors.New("usage")

// command is one subcommand of booster.
type command struct {
	name    string
	args    string // the arguments, for the usage text
	summary string
	run     func(env, []string) error
}

// env is what the commands get from main.
type env struct {
	page fs.FS // the built page of the window
}

func commands() []command {
	return []command{
		{"window", "[-dir D] [-x X -y Y]", "open the window (the default); start the core if none runs", runWindow},
		{"core", "", "run the core in the foreground", runCore},
		{"list", "", "list the core's sessions", runList},
		{"new", "[-kind claude|pi|shell] [-dir D]", "start a session and print its id", runNew},
		{"prompt", "[-submit=false] <id> <text | ->", "send an Action prompt to a session", runPrompt},
		{"close", "<id>", "end a session", runClose},
		{"stop", "", "stop the core and every session", runStop},
		{"project", "<command> ...", "manage the Projects (booster project help)", runProject},
	}
}

// Run runs the command that args (os.Args[1:]) name. With no command, or
// with a flag first, it opens the window.
func Run(args []string, page fs.FS) error {
	name := "window"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	return dispatch("booster", commands(), env{page: page}, name, args)
}

// dispatch runs the command of the table cmds that name names; prog is what
// the usage text calls the table.
func dispatch(prog string, cmds []command, e env, name string, args []string) error {
	if name == "help" {
		usage(os.Stdout, prog, cmds)
		return nil
	}
	for _, c := range cmds {
		if c.name == name {
			return c.run(e, args)
		}
	}
	fmt.Fprintf(os.Stderr, "%s: unknown command %q\n\n", prog, name)
	usage(os.Stderr, prog, cmds)
	return ErrUsage
}

// usageError reports a wrong command line on stderr and returns ErrUsage.
func usageError(format string, a ...any) error {
	fmt.Fprintf(os.Stderr, "booster: "+format+"\n", a...)
	return ErrUsage
}

// parse parses a command's flags. The flag package reports a wrong flag
// itself; -h and -help return flag.ErrHelp.
func parse(flags *flag.FlagSet, args []string) error {
	err := flags.Parse(args)
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		return ErrUsage
	}
	return err
}

func usage(w io.Writer, prog string, cmds []command) {
	fmt.Fprintf(w, "Usage: %s [command] [arguments]\n\n", prog)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, c := range cmds {
		fmt.Fprintf(tw, "  %s %s\t%s\n", c.name, c.args, c.summary)
	}
	fmt.Fprintf(tw, "  help\tshow this text\n")
	_ = tw.Flush()
}
