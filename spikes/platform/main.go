// Command platform is the booster platform spike: one Wails 3 window with
// xterm.js terminals on real PTYs, and benchmarks that measure how the
// terminal renders in the system webview.
package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	// Chooses the integrated GPU before Wails reads the environment.
	_ "booster/spikes/platform/webviewgpu"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == generatorCommand {
		os.Exit(runGenerator(os.Args[2:]))
	}

	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("webview GL: EGL vendors %q, DMA-BUF renderer off %q",
		os.Getenv("__EGL_VENDOR_LIBRARY_FILENAMES"), os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER"))

	srv, err := newServer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:   "booster-platform-spike",
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	srv.quit = app.Quit

	go func() {
		if err := srv.serve(); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "booster platform spike",
		Width:            1400,
		Height:           900,
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              "/?" + srv.frontendQuery().Encode(),
		DevToolsEnabled:  true,
		Linux:            application.LinuxWindow{WebviewGpuPolicy: gpuPolicies[cfg.gpuPolicy]},
	})
	if cfg.placed() {
		// Wails places a new window relative to the monitor it starts on; the
		// spike needs an absolute place to choose the monitor.
		win.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
			win.SetPosition(cfg.x, cfg.y)
		})
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
