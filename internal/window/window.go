// Package window is booster's Wails window. The page talks to the core
// directly over loopback TCP (see package core), so the window carries no
// bindings and relays nothing; closing it leaves the core and its sessions
// running.
package window

import (
	"io/fs"
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/stevetosak/booster/internal/core"
)

// Options configure the window.
type Options struct {
	// Page is the built page: index.html and its assets.
	Page fs.FS
	// Core tells the page where the core listens and which token it takes.
	Core core.Info
	// Dir is the folder the page offers for new sessions.
	Dir string
	// X and Y place the window; a negative value leaves it to the window
	// manager.
	X, Y int
}

// Run opens the window and returns when it closes.
func Run(opts Options) error {
	app := application.New(application.Options{
		Name:   "booster",
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(opts.Page)},
	})
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "booster",
		Width:            1400,
		Height:           900,
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              pageURL(opts),
		DevToolsEnabled:  true,
	})
	if opts.X >= 0 && opts.Y >= 0 {
		win.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) { win.SetPosition(opts.X, opts.Y) })
	}
	return app.Run()
}

// pageURL hands the core's address and token to the page in its query.
func pageURL(opts Options) string {
	q := url.Values{"core": {"http://" + opts.Core.Addr}, "token": {opts.Core.Token}, "dir": {opts.Dir}}
	return "/?" + q.Encode()
}
