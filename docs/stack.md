---
title: "booster: the stack"
description: "Options and the choice for each layer of the app: the webview, how much Code-OSS to build on, the editor, the terminal, the core, the UI framework and the classic IDE services."
type: design
status: proposed
topics: [ide, stack]
---

booster is its own desktop app on Wails 3 with a Go core in the same binary,
a React frontend, and an own session host (decisions of 2026-11-19). This
page records how each layer was weighed. Versions were read from the npm
registry, crates.io and the Go module proxy on 2026-11-19.

## The shape

```
shell (Wails 3, Go, thin)         windows, menus, native dialogs, starts the core
  └─ web frontend (TypeScript)    the IDE: layout, tool windows, editor, terminals, Board
        │  one socket protocol
booster core (Go, same binary)    Projects, session host, Actions, config, analytics,
                                  Board, git, search, LSP and DAP brokering
  └─ agent PTYs, language servers, debug adapters, git, ripgrep
```

The core outlives the window, because it owns the agent sessions. The
frontend reaches native features through one `Platform` interface, so the
webview runtime (Tauri with WebKitGTK, Tauri with CEF, or Electron) is a
replaceable adapter and not a property of the IDE code.

## The webview on Linux

Tauri 2 (2.12.7 stable) uses the system webview: WebKitGTK on Linux, WebView2
on Windows, WKWebView on macOS. Tauri's own Linux graphics page says WebGL and
canvas can "silently land on a slow path", seen as "high input latency or low
frame rates in WebGL heavy views (terminal emulators, editors …)", most often
on NVIDIA, and WebKitGTK hides the real renderer, so the app cannot detect it.
Its workarounds (`WEBKIT_DISABLE_DMABUF_RENDERER=1`, then disabling
compositing) give up the fast path. The Operator's laptop runs WebKitGTK
2.50.4 on X11, with the internal panel on the AMD GPU and the external
monitor on the NVIDIA GPU.

Full terminals are the condition of the GUI decision, so this is the first
risk to measure. If WebKitGTK fails it, two fallbacks keep the frontend as it
is: Tauri 3's CEF runtime (`tauri-runtime-cef` 3.0.0-alpha.7, alpha; it ships
Chromium, hundreds of MB larger), or Electron (45.4.1), which ships Chromium
and Node together.

## Tauri or Wails

Wails is the Go counterpart of Tauri: a Go backend, the system webview, and
generated TypeScript bindings for Go methods. Wails v3 became stable on
2026-10-26 (3.1.0 on 2026-11-01) with multiple windows, services, typed
events, menus, a tray and an MCP service that lets an agent drive and inspect
a running app. On Linux it uses GTK4 and WebKitGTK 6.0 by default; the GTK3
stack was kept behind a build tag "until v3.1". It has no Chromium runtime.
The GTK4 backend needs GTK 4.10+ (`GtkFileDialog`, `gdk_monitor_get_scale`)
and does not build on Ubuntu 22.04 (GTK 4.6.9), so booster builds with
`-tags gtk3` there; the tag may go in a later Wails release.

| | Tauri 2 | Wails 3 |
|---|---|---|
| Shell language | Rust | Go |
| Linux webview | WebKitGTK, GTK3 API | WebKitGTK, GTK4 API (webkitgtk-6.0) |
| Chromium fallback | CEF runtime in Tauri 3 (alpha) | None; Electron only |
| Maturity | Stable since 2024, large plugin set | Stable for three weeks |
| On the laptop | Rust not installed | Go 1.26.2 installed (v3 needs 1.26+); `libwebkitgtk-6.0-4` 2.50.4 available, GTK 4.6.9 |

Both run the same WebKit engine on Linux (2.50.4 on the laptop for either
API), so the terminal risk below is shared; the GTK4 path renders through a
different toolkit, which can change the result in either direction. With a
thin shell behind the `Platform` interface and the core in its own process,
the shell language carries little code, so the choice between them is cheap
to reverse. The difference that counts is the fallback: Tauri keeps a
Chromium path inside the same framework, Wails does not.

## How much Code-OSS

| Level | What it means | Gain | Cost |
|---|---|---|---|
| 1. Fork Code-OSS | The Cursor path, on Electron | Everything works on day one | Electron, not Tauri; the VS Code look; a monthly merge of a very large code base |
| 2. The Code-OSS workbench in Tauri | SideX, vscodeee, or the workbench through `monaco-vscode-api` | The whole workbench, the extension UI, SCM, search | Every project is early (SideX: "the extension host and debugger are still in progress"); the extension host needs a Node sidecar; booster's own UI must fit the workbench model; the VS Code look stays |
| 3. Code-OSS parts as libraries | Own shell and layout; Monaco as the editor; chosen services from `@codingame/monaco-vscode-api` | VS Code's editor quality: diff editor, peek, inlay hints, code lens, find, multi-cursor; TextMate grammars and themes; LSP wiring that maps 1:1 | Monaco is large and has VS Code's own style of extension points; the shell, tool windows and debugger UI are booster's own work |
| 4. No Code-OSS | Own shell with CodeMirror 6 | Small, modular, simple to style; own LSP glue | Diff, peek, inlay hints, code lens and the language wiring are own work |

Level 3 is the decision (2026-11-22). The JetBrains feel comes from the shell (tool
windows, Search Everywhere, keymaps, the debugger and run windows), not from
the editor widget, so the shell is booster's own; the editor is where VS
Code's ten years of work are worth taking as they are. Monaco's diff editor
is also the natural base for the later review Plan. `@codingame/monaco-vscode-api`
(39.0.5, released 2026-11-18) is active and follows VS Code's releases; it is
adopted service by service (grammars, themes and languages first), never the
whole workbench.

## The terminal

xterm.js (6.0.0) is the terminal VS Code uses, and `@xterm/headless` with the
serialize addon is the same emulator without a screen, which makes the session
host's screen restore exact. Its fast renderer is WebGL, which is the slow
path above; the platform spike measures the WebGL and DOM renderers on both
GPUs.

## The core

| Option | Gain | Cost |
|---|---|---|
| Go, the same binary as a Wails shell (`booster core`) | The Operator's preferred language; one binary to ship, no runtime; `creack/pty` (1.1.24) and `charmbracelet/x/vt` (0.1.0, with `Render()` of the screen as ANSI and scrollback access) | The emulator is young and differs from xterm.js, so a restored screen may differ in details; scrollback-to-ANSI is own code; kalfa is used through its CLI, not its code |
| Node 24 service in TypeScript | `node-pty` (1.1.0) with `@xterm/headless`: the same emulator on both sides, exact restore with the serialize addon; kalfa and symbiosis code reusable | A second language beside the shell; a Node runtime to ship |
| Rust inside or beside Tauri | One language with a Tauri shell; `portable-pty` (0.9.0), `alacritty_terminal` (0.26.0) | Own serialization from a different emulator; not the preferred language |

Go is chosen: the Operator prefers it, and one binary is simpler to
ship once booster is shared. Only the live bytes must be exact, and they pass
through raw; the emulator serves only the restore, which the session spike
compares against `@xterm/headless`. TypeScript stays where the host decides
it: the frontend and the pi extension.

## The classic IDE services

- **Go to definition and code intelligence:** language servers started by the
  core, wired to Monaco through `monaco-languageclient`; the symbiosis
  tree-sitter outlines serve as a fallback where no server runs.
- **Debugger:** the Debug Adapter Protocol, with the adapters VS Code uses
  (js-debug, debugpy, CodeLLDB, netcoredbg), and an own debugger tool window.
- **Git:** the `git` CLI, as JetBrains and VS Code use it, so hooks, config and
  credentials behave as in a terminal; `gix` (0.90.0) only if status becomes
  slow on large repositories.
- **Search:** ripgrep for text; a fuzzy matcher for Search Everywhere.
- **Run configurations:** an own model, with symbiosis's launch.json import.

## The UI framework

React with TypeScript, chosen over Solid and Svelte 5 for its ecosystem
(docking layouts, virtualised lists and trees for an IDE shell). The editor
and the terminal are imperative widgets that React mounts and does not
re-render.
