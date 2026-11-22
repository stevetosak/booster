# Platform spike

Part 1 of [the booster plan](../../docs/plan.md): does a full terminal
(xterm.js 6) render well enough in Wails 3's system webview (WebKitGTK) on
the Operator's laptop, on the AMD panel and on the NVIDIA monitor?

One Wails window runs xterm.js terminals on real PTYs. The Go side serves the
terminals and a control channel over WebSockets on loopback; each connection
must carry a random token that only the window's URL holds. The page names a
terminal kind (`main`, `cat`, `gen`), never a program.

## Build

Ubuntu 22.04 has GTK 4.6, and Wails' default GTK4 backend needs 4.10+, so the
Taskfile builds the GTK3 backend (`-tags gtk3`, WebKitGTK 4.1). It needs
`libwebkit2gtk-4.1-dev`.

```sh
wails3 task build        # bin/platform
```

## Run

```sh
bin/platform -cmd claude -dir ~/Projects/Active/booster   # or -cmd pi
bin/platform -renderer dom                                # the DOM renderer
```

The Terminal view runs the main program. The Benchmarks view runs the suite
and saves a JSON report:

- **Throughput:** 32 MiB of plain text and 32 MiB of coloured, styled text at
  full speed; MB/s from the program start to the last byte drawn, and the frame
  times during the stream.
- **Typing latency:** keys at a typing pace into `cat` (the kernel echoes them),
  timed to the echo parsed, drawn, and the next frame; idle, and beside a
  second terminal that streams 256 KB/s (an agent) or 2 MB/s (a build log).
- **Memory:** PSS of the app and WebKit's helper processes, idle and under load.

The latency starts at xterm.js's input, so it leaves out the path from the
keyboard through GTK and WebKit to the page; the Operator's own check covers it.

## The grid

```sh
./grid.sh                    # every monitor × default GL / NVIDIA offload × WebGL / DOM
./summary.sh reports/<run>   # one Markdown table
```

## The one-update-late check

On the laptop the window showed each terminal update one update late, while
the page had drawn it within 20 ms (`-trace` logs input and output events).
`webkitcheck` is one WebKitGTK 4.1 window without Wails, to tell whether
WebKitGTK or the embedding app causes it: its page counter must match the
window title's.

```sh
gcc -O2 -o bin/webkitcheck webkitcheck/webkitcheck.c $(pkg-config --cflags --libs webkit2gtk-4.1)
bin/webkitcheck            # hardware acceleration always, as Wails sets it
bin/webkitcheck never
bin/platform -gpu-policy never
```
