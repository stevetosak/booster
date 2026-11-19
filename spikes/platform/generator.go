package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// generatorCommand is the first argument that runs this binary as the output
// generator of the throughput benchmarks, inside a PTY.
const generatorCommand = "gen"

// runGenerator writes terminal output: `gen -kind plain|color -bytes N -rate KB/s`.
// With -bytes 0 it writes until the terminal closes.
func runGenerator(args []string) int {
	fs := flag.NewFlagSet(generatorCommand, flag.ContinueOnError)
	kind := fs.String("kind", "plain", "plain or color")
	total := fs.Int64("bytes", 8<<20, "bytes to write; 0 writes until the terminal closes")
	rate := fs.Int64("rate", 0, "kilobytes per second; 0 is unlimited")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	line, err := lineMaker(*kind)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	out := bufio.NewWriterSize(os.Stdout, 64<<10)
	start := time.Now()
	var written int64
	for i := 0; *total == 0 || written < *total; i++ {
		n, err := out.WriteString(line(i))
		if err != nil {
			return 1 // the terminal closed
		}
		written += int64(n)
		if *rate > 0 {
			if err := pace(out, start, written, *rate); err != nil {
				return 1
			}
		}
	}
	if err := out.Flush(); err != nil {
		return 1
	}
	return 0
}

// pace flushes and sleeps while the output runs ahead of the rate.
func pace(out *bufio.Writer, start time.Time, written, kbPerSec int64) error {
	due := time.Duration(written * int64(time.Second) / (kbPerSec * 1024))
	if ahead := due - time.Since(start); ahead > time.Millisecond {
		if err := out.Flush(); err != nil {
			return err
		}
		time.Sleep(ahead)
	}
	return nil
}

func lineMaker(kind string) (func(int) string, error) {
	switch kind {
	case "plain":
		return func(i int) string {
			return fmt.Sprintf("%08d the quick brown fox jumps over the lazy dog 0123456789 abcdefghijklmnopqrstuvwxyz\n", i)
		}, nil
	case "color":
		return colorLine, nil
	default:
		return nil, fmt.Errorf("unknown -kind %q: use plain or color", kind)
	}
}

var words = []string{"func", "return", "agent", "session", "booster", "render", "terminal", "webview"}

// colorLine mixes 256-colour, true-colour, bold and reset sequences, as a
// syntax-highlighted diff or an agent's styled output does.
func colorLine(i int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[2m%08d\x1b[0m ", i)
	for j, w := range words {
		switch (i + j) % 3 {
		case 0:
			fmt.Fprintf(&b, "\x1b[38;5;%dm%s\x1b[0m ", (i*7+j*31)%256, w)
		case 1:
			fmt.Fprintf(&b, "\x1b[1;38;2;%d;%d;%dm%s\x1b[0m ", (i*3)%256, (j*40)%256, 200, w)
		default:
			fmt.Fprintf(&b, "\x1b[48;5;%dm%s\x1b[0m ", 232+(i+j)%24, w)
		}
	}
	b.WriteString("\n")
	return b.String()
}
