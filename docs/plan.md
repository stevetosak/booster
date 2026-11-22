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

## Later parts

Shaped after the two spikes, each with its own Done when line: the core
skeleton (Projects, Sessions, the Board from kalfa's Task Records), the shell
and the editor, Actions with the first default Action ("Generate run
configuration"), agentic config, usage analytics from kalfa's meter with
optional Langfuse export, and the classic IDE services.
