package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// gpuPolicies maps the -gpu-policy values to WebKit's hardware acceleration policies.
var gpuPolicies = map[string]application.WebviewGpuPolicy{
	"always":   application.WebviewGpuPolicyAlways,
	"ondemand": application.WebviewGpuPolicyOnDemand,
	"never":    application.WebviewGpuPolicyNever,
}

// config holds the command-line choices of one spike run.
type config struct {
	cmd       string // command of the main terminal
	dir       string // working folder of the main terminal
	renderer  string // terminal renderer: "webgl" or "dom"
	autobench string // report path; empty means an interactive run
	label     string // free text recorded in the report
	trace     bool   // log the main terminal's input and output events
	x, y      int    // absolute window position; negative keeps the default
	gpuPolicy string // WebKit's hardware acceleration policy: always, ondemand or never
}

func parseConfig(args []string) (config, error) {
	var c config
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return c, fmt.Errorf("find the home folder: %w", err)
	}

	fs := flag.NewFlagSet("platform", flag.ContinueOnError)
	fs.StringVar(&c.cmd, "cmd", shell, "command of the main terminal, for example claude or pi")
	fs.StringVar(&c.dir, "dir", home, "working folder of the main terminal")
	fs.StringVar(&c.renderer, "renderer", "webgl", "terminal renderer: webgl or dom")
	fs.StringVar(&c.autobench, "autobench", "", "run the benchmarks, write the report to this path, and quit")
	fs.StringVar(&c.label, "label", "", "label recorded in the report")
	fs.BoolVar(&c.trace, "trace", false, "write the main terminal's input and output events to the log (stderr)")
	fs.StringVar(&c.gpuPolicy, "gpu-policy", "always", "WebKit hardware acceleration: always, ondemand or never")
	fs.IntVar(&c.x, "x", -1, "absolute window x position")
	fs.IntVar(&c.y, "y", -1, "absolute window y position")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if c.renderer != "webgl" && c.renderer != "dom" {
		return c, fmt.Errorf("-renderer must be webgl or dom, not %q", c.renderer)
	}
	if _, ok := gpuPolicies[c.gpuPolicy]; !ok {
		return c, fmt.Errorf("-gpu-policy must be always, ondemand or never, not %q", c.gpuPolicy)
	}
	return c, nil
}

func (c config) placed() bool { return c.x >= 0 && c.y >= 0 }
