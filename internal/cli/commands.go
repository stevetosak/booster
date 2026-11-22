package cli

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/stevetosak/booster/internal/core"
	"github.com/stevetosak/booster/internal/project"
	"github.com/stevetosak/booster/internal/session"
	"github.com/stevetosak/booster/internal/session/ptyhost"
	"github.com/stevetosak/booster/internal/window"

	// Its init chooses the integrated GPU before Wails reads the environment;
	// OriginalEnviron keeps the environment as it was for the core and the
	// agents.
	"github.com/stevetosak/booster/internal/webviewgpu"
)

func runWindow(e env, args []string) error {
	flags := flag.NewFlagSet("window", flag.ContinueOnError)
	dir := flags.String("dir", homeDir(), "folder for new sessions")
	x := flags.Int("x", -1, "absolute window x position")
	y := flags.Int("y", -1, "absolute window y position")
	if err := parse(flags, args); err != nil {
		return err
	}
	log.Printf("webview GL: EGL vendors %q, DMA-BUF renderer off %q",
		os.Getenv("__EGL_VENDOR_LIBRARY_FILENAMES"), os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER"))
	self, err := os.Executable()
	if err != nil {
		return err
	}
	info, err := core.Ensure(core.DefaultPaths(), self, webviewgpu.OriginalEnviron())
	if err != nil {
		return err
	}
	log.Printf("core %d at %s", info.PID, info.Addr)
	return window.Run(window.Options{Page: e.page, Core: info, Dir: *dir, X: *x, Y: *y})
}

func runCore(_ env, args []string) error {
	if err := parse(flag.NewFlagSet("core", flag.ContinueOnError), args); err != nil {
		return err
	}
	registry, err := project.DefaultPath()
	if err != nil {
		return err
	}
	srv, err := core.Listen(core.Config{
		Paths:      core.DefaultPaths(),
		Backend:    ptyhost.New(ptyhost.Options{Programs: programs(), Env: webviewgpu.OriginalEnviron()}),
		Projects:   project.NewRegistry(registry),
		PageOrigin: window.PageOrigin,
	})
	if err != nil {
		return err
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	signal.Ignore(syscall.SIGHUP) // the core outlives the terminal that started it
	go func() { <-sig; srv.Stop() }()
	log.Printf("core %d listening: %s", os.Getpid(), srv)
	srv.Serve()
	log.Printf("core %d stopped", os.Getpid())
	return nil
}

// programs maps the session kinds to command lines.
func programs() map[session.Kind][]string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return map[session.Kind][]string{
		session.KindClaude: {"claude"},
		session.KindPi:     {"pi"},
		session.KindShell:  {shell, "-l"},
	}
}

func runList(_ env, args []string) error {
	if err := parse(flag.NewFlagSet("list", flag.ContinueOnError), args); err != nil {
		return err
	}
	list, err := client().List()
	if err != nil {
		return err
	}
	for _, s := range list {
		fmt.Printf("%s  %-6s  %3dx%-3d  clients %d  %s  %q\n", s.ID, s.Kind, s.Size.Cols, s.Size.Rows, s.Clients, s.Dir, s.Title)
	}
	return nil
}

func runNew(_ env, args []string) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	kind := flags.String("kind", string(session.KindShell), "claude, pi or shell")
	dir := flags.String("dir", homeDir(), "working folder")
	if err := parse(flags, args); err != nil {
		return err
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	info, err := client().Create(session.Spec{Kind: session.Kind(*kind), Dir: abs})
	if err != nil {
		return err
	}
	fmt.Println(info.ID)
	return nil
}

func runPrompt(_ env, args []string) error {
	flags := flag.NewFlagSet("prompt", flag.ContinueOnError)
	submit := flags.Bool("submit", true, "press Enter after the prompt")
	if err := parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return usageError("usage: booster prompt [-submit=false] <id> <text | ->")
	}
	text := flags.Arg(1)
	if text == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		text = strings.TrimRight(string(b), "\n")
	}
	return client().Prompt(flags.Arg(0), text, *submit)
}

func runClose(_ env, args []string) error {
	if len(args) != 1 {
		return usageError("usage: booster close <id>")
	}
	return client().Close(args[0])
}

func runStop(_ env, args []string) error {
	if len(args) != 0 {
		return usageError("usage: booster stop")
	}
	return client().Shutdown()
}

// client reaches the running core over its Unix socket.
func client() *core.Client { return core.NewClient(core.DefaultPaths()) }

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return home
}
