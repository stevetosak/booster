---
title: "booster: an IDE for the agentic workflow"
description: "One project-based desktop IDE (Wails 3, Go) with a JetBrains feel that hosts Claude Code and pi sessions, Actions, agentic config, usage analytics and a Task board; two spikes test the Linux webview and the session host first."
type: plan
status: proposed
topics: [ide, agents]
---

booster replaces the pair of Herdr (agent panes) and ttt with symbiosis (code
reading) with one IDE. It hosts the agentic workflow (agent sessions in full
terminals, Actions, agentic config, usage analytics, a Task board) and the
classic IDE side (git, diff review, debugger, search, go to definition, run
configurations), and it opens projects, not folders. It is the Operator's
daily tool first and is shared once it is good. Better code review for the AI
era is a later Plan, built on top of this one.

**Done when:** the Operator does one full working week only in booster, with
no separate Herdr or ttt window.

## Decisions

| # | Question | Decision |
|---|---|---|
| 1 | Audience | The Operator's daily tool first; shared with others once it is good |
| 2 | GUI or terminal | A GUI, on the condition that full terminals run inside it |
| 3 | Agents on day one | Claude Code and pi |
| 4 | Host | Its own desktop app (first named as Tauri; see 9) |
| 5 | Session backend | An own session host behind a `SessionBackend` interface; tmux may come later as a second backend |
| 6 | Shape | A thin shell, a web frontend behind a `Platform` interface, and a headless core that owns the sessions (see [stack.md](stack.md)) |
| 7 | Code-OSS | Level 3, parts as libraries: own shell and layout, Monaco as the editor, chosen `monaco-vscode-api` services; not the workbench, not a fork (2026-11-22) |
| 8 | Core language | Go, the same binary as the shell (`booster core`); the Operator prefers Go and is open to Rust |
| 9 | Shell | Wails 3: it passed the platform spike, so Tauri 2 and Electron were not measured (2026-11-22) |
| 10 | UI framework | React with TypeScript; the editor and the terminal are imperative widgets inside it |
| 11 | Wails on Ubuntu 22.04 | The GTK3 build tag (WebKitGTK 4.1): the default GTK4 backend needs GTK 4.10+, the laptop has 4.6.9 (2026-11-22) |
| 12 | How clients reach the core | One HTTP/WebSocket API on two listeners: a Unix socket (mode 0600) for local tools, and loopback TCP with a token for the window's page, which talks to the core directly (2026-11-22) |
| 13 | Scope of Part 3 | The core and a minimal window; Part 3 ends when booster replaces Herdr for a day of work, Part 4 when it replaces ttt; kalfa's herdr Module sees only Herdr until a later part (2026-11-22) |
| 14 | Agents across a core restart | The core owns the PTYs; each agent gets a session id at start and is started again with resume after a core restart; a development build runs as a separate instance; a holder process per session stays a later option behind `session.Backend` (2026-11-22) |
| 15 | How booster knows a Project | An own registry (`$XDG_CONFIG_HOME/booster/projects.json`: name and main checkout), worktrees from `git worktree list`, the kalfa project key computed as kalfa does, a one-time import of `~/Projects/Active` (2026-11-22) |
| 16 | How the Board reads kalfa | Through kalfa's CLI, with new `--json` output; booster does not read kalfa's files (2026-11-22) |

## 1. Platform spike

**Done when:** one xterm.js frontend runs Claude Code and pi in Wails 3
(WebKitGTK 4.1, GTK3; see 11) on the Operator's laptop, measured on both the AMD panel and
the NVIDIA monitor with the WebGL and DOM renderers; if it lags, the same
frontend is measured in Tauri 2 (WebKitGTK, GTK3) and Electron; the Operator
picks the shell.

- [x] One minimal frontend with one xterm.js terminal on a local PTY, in Wails 3.
- [x] Input latency and output throughput, per renderer and per GPU, in one table ([stack.md](stack.md#measured-on-the-laptop-2026-11-22)).
- [x] The Operator's own check of Claude Code and pi in the window, on both monitors. The first check showed each update one update late, because Wails turns WebKit's DMA-BUF renderer off on this laptop; with the webview on the AMD GPU and DMA-BUF on, both agents run with no lag.
- [x] The Operator's choice recorded in the Decisions (9). Wails did not lag, so Tauri 2 and Electron, with their memory, were not measured.

## 2. Session spike

**Done when:** a prototype session host keeps one Claude Code and one pi
session alive across a restart of the window, restores the screen on
reattach, keeps xterm.js scrollback and search, and delivers an Action prompt
to each agent.

- [x] A Go session host on `creack/pty` and `charmbracelet/x/vt` with a socket protocol (`spikes/session`, see 12).
- [x] The restored screen and scrollback compared with `@xterm/headless` on the same output ([stack.md](stack.md#measured-the-session-spike-2026-11-22)).
- [x] The spike app attaches to it, closes, opens and attaches again, with a shell, Claude Code and pi (the Operator's check). A search finds a shell line restored from the scrollback; in the fullscreen agents it holds only for the screen as it is, so a search over an agent's output comes from its transcript in a later part.
- [x] An Action prompt reaches Claude Code and pi and starts a turn in each (the Operator's check).

## 3. Core skeleton

**Done when:** the Operator does one full day of agent work in booster with
no Herdr: Projects open by name, Claude Code and pi sessions run per Project
in tabs and survive a restart of the window, and the Board shows the Tasks of
every Project.

The core leaves the spike and becomes `booster core`, the long-lived service
in the booster binary. The window stays minimal; the IDE layout comes with the
shell and the editor in Part 4, which ends when booster replaces ttt. During
this part kalfa's herdr Module (`kalfa board`, the successor tab, `kalfa
space`) still sees only Herdr (see 13).

- [x] The code tree: one Go module at the repository root with `booster` (the window) and `booster core`; the spike's session host, core and window move in and are cleaned, and `spikes/` stays as the record. The build is a Makefile, not Wails' generated Taskfiles, and CI runs gofmt, vet, the race tests and the build on GTK3.
- [x] Projects: the registry, the worktrees, the kalfa project key and the one-time import (see 15). `booster project list|add|remove|import` and the API under `/api/projects`; the key matched kalfa's own `projectKey` for each of the 10 checkouts in `~/Projects/Active`. It differs from kalfa only on a remote URL with `..` in its path, an IPv6 host or a non-ASCII host, which no remote has.
- [ ] Sessions: a session id per agent, the session list saved, resume after a core restart, a separate instance for development builds (see 14); the agent's state (working, idle, waiting for the Operator) is shaped as its own design question.
- [ ] `--json` output in kalfa for `task list`, `task show`, `plan` and `defer`, in the kalfa repository (see 16).
- [ ] The Board: the Tasks of every Project, from kalfa's CLI.
- [ ] The minimal window: a Project switcher, session tabs per Project, the Board as a list.
- [ ] The Operator's one-day check without Herdr.

## Later parts

Shaped after Part 3, each with its own Done when line: the shell and the
editor (Part 4, done when booster replaces ttt), Actions with the first
default Action ("Generate run configuration"), agentic config, usage
analytics from kalfa's meter with optional Langfuse export, the classic IDE
services, and a kalfa Module that reaches booster's sessions as the herdr
Module reaches Herdr's.
