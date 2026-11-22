// Command booster is an IDE for the agentic workflow. One binary is the
// window, the core that owns the agent sessions, and the tools that talk to a
// running core; `booster help` lists the commands (package internal/cli).
package main

import (
	"embed"
	"errors"
	"flag"
	"io/fs"
	"log"
	"os"

	"github.com/stevetosak/booster/internal/cli"
)

// dist is the built page (npm run build in frontend/).
//
//go:embed all:frontend/dist
var dist embed.FS

func main() {
	page, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	switch err := cli.Run(os.Args[1:], page); {
	case err == nil:
	case errors.Is(err, flag.ErrHelp):
		os.Exit(0)
	case errors.Is(err, cli.ErrUsage):
		os.Exit(2)
	default:
		log.Fatal(err)
	}
}
