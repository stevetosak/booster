// Command session is the booster session spike: a core process that keeps
// Claude Code and pi sessions alive while the window closes and opens again,
// a Wails window that attaches to them, and tools that test the screen
// restore and Action prompts.
//
//	session [window] [-dir D] [-x X -y Y]   open the window; start the core if none runs
//	session core [-record DIR]              run the core in the foreground
//	session list | new | prompt | close | stop
//	session replay -raw F.raw               the snapshot x/vt makes from a recording
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"booster/spikes/session/core"
	"booster/spikes/session/session"
	"booster/spikes/session/session/ptyhost"

	// Chooses the integrated GPU before Wails reads the environment, and keeps
	// the environment as it was for the core and the agents.
	"booster/spikes/session/webviewgpu"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cmd, args := "window", os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	commands := map[string]func([]string) error{
		"window": runWindow,
		"core":   runCore,
		"list":   runList,
		"new":    runNew,
		"prompt": runPrompt,
		"close":  runClose,
		"stop":   runStop,
		"replay": runReplay,
	}
	run, ok := commands[cmd]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command %q; see the package comment in main.go\n", cmd)
		os.Exit(2)
	}
	if err := run(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		log.Fatal(err)
	}
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return home
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

func runCore(args []string) error {
	fs := flag.NewFlagSet("core", flag.ContinueOnError)
	record := fs.String("record", "", "folder to record each session's output into (for replay and compare)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := ptyhost.Options{Programs: programs(), Env: webviewgpu.OriginalEnviron()}
	if *record != "" {
		opts.Record = ptyhost.RecordTo(*record)
	}
	srv, err := core.Listen(core.DefaultPaths(), ptyhost.New(opts), pageOrigin)
	if err != nil {
		return err
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	signal.Ignore(syscall.SIGHUP)
	go func() { <-sig; srv.Stop() }()
	log.Printf("core %d listening: %s; recording: %q", os.Getpid(), srv, *record)
	srv.Serve()
	log.Printf("core %d stopped", os.Getpid())
	return nil
}

func runWindow(args []string) error {
	fs := flag.NewFlagSet("window", flag.ContinueOnError)
	dir := fs.String("dir", homeDir(), "folder for new sessions")
	x := fs.Int("x", -1, "absolute window x position")
	y := fs.Int("y", -1, "absolute window y position")
	if err := fs.Parse(args); err != nil {
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

	app := application.New(application.Options{
		Name:   "booster-session-spike",
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	q := "/?" + url.Values{"core": {"http://" + info.Addr}, "token": {info.Token}, "dir": {*dir}}.Encode()
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "booster session spike",
		Width:            1400,
		Height:           900,
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              q,
		DevToolsEnabled:  true,
	})
	if *x >= 0 && *y >= 0 {
		win.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) { win.SetPosition(*x, *y) })
	}
	return app.Run()
}

func runList([]string) error {
	list, err := core.NewClient(core.DefaultPaths()).List()
	if err != nil {
		return err
	}
	for _, s := range list {
		fmt.Printf("%s  %-6s  %3dx%-3d  clients %d  %s  %q\n", s.ID, s.Kind, s.Size.Cols, s.Size.Rows, s.Clients, s.Dir, s.Title)
	}
	return nil
}

func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	kind := fs.String("kind", "shell", "claude, pi or shell")
	dir := fs.String("dir", homeDir(), "working folder")
	if err := fs.Parse(args); err != nil {
		return err
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	info, err := core.NewClient(core.DefaultPaths()).Create(session.Spec{Kind: session.Kind(*kind), Dir: abs})
	if err != nil {
		return err
	}
	fmt.Println(info.ID)
	return nil
}

func runPrompt(args []string) error {
	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	submit := fs.Bool("submit", true, "press Enter after the prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: session prompt [-submit=false] <id> <text | ->")
	}
	text := fs.Arg(1)
	if text == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		text = strings.TrimRight(string(b), "\n")
	}
	return core.NewClient(core.DefaultPaths()).Prompt(fs.Arg(0), text, *submit)
}

func runClose(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: session close <id>")
	}
	return core.NewClient(core.DefaultPaths()).Close(args[0])
}

func runStop([]string) error { return core.NewClient(core.DefaultPaths()).Shutdown() }

func runReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	rawPath := fs.String("raw", "", "the recorded output (<id>.raw); its <id>.json sits beside it")
	scrollback := fs.Int("scrollback", 10000, "scrollback lines")
	if err := fs.Parse(args); err != nil {
		return err
	}
	raw, err := os.ReadFile(*rawPath)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(strings.TrimSuffix(*rawPath, ".raw") + ".json")
	if err != nil {
		return err
	}
	var rec ptyhost.Recording
	if err := json.Unmarshal(b, &rec); err != nil {
		return err
	}
	_, err = os.Stdout.Write(ptyhost.Replay(raw, rec, *scrollback))
	return err
}
