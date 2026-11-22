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
| 9 | Shell | Wails 3 first; Tauri 2 and Electron only if Wails fails the platform spike |
| 10 | UI framework | React with TypeScript; the editor and the terminal are imperative widgets inside it |
| 11 | Wails on Ubuntu 22.04 | The GTK3 build tag (WebKitGTK 4.1): the default GTK4 backend needs GTK 4.10+, the laptop has 4.6.9 (2026-11-22) |

## 1. Platform spike

**Done when:** one xterm.js frontend runs Claude Code and pi in Wails 3
(WebKitGTK 4.1, GTK3; see 11) on the Operator's laptop, measured on both the AMD panel and
the NVIDIA monitor with the WebGL and DOM renderers; if it lags, the same
frontend is measured in Tauri 2 (WebKitGTK, GTK3) and Electron; the Operator
picks the shell.

- [x] One minimal frontend with one xterm.js terminal on a local PTY, in Wails 3.
- [x] Input latency and output throughput, per renderer and per GPU, in one table ([stack.md](stack.md#measured-on-the-laptop-2026-11-22)).
- [ ] The Operator's own check of Claude Code and pi in the window, on both monitors: the window shows each update one update late (cause open).
- [ ] Only if Wails lags: the same frontend in Tauri 2 and in Electron.
- [ ] The memory of the same frontend in Tauri (WebKitGTK) and in Electron, counted with PSS so shared pages count once.
- [ ] The Operator's choice recorded in the Decisions.

## 2. Session spike

**Done when:** a prototype session host keeps one Claude Code and one pi
session alive across a restart of the window, restores the screen on
reattach, keeps xterm.js scrollback and search, and delivers an Action prompt
to each agent.

- [ ] A Go session host on `creack/pty` and `charmbracelet/x/vt` with a socket protocol.
- [ ] The restored screen and scrollback compared with `@xterm/headless` on the same output.
- [ ] The spike app attaches to it, closes, opens and attaches again.
- [ ] An Action prompt reaches Claude Code and pi.

## Later parts

Shaped after the two spikes, each with its own Done when line: the core
skeleton (Projects, Sessions, the Board from kalfa's Task Records), the shell
and the editor, Actions with the first default Action ("Generate run
configuration"), agentic config, usage analytics from kalfa's meter with
optional Langfuse export, and the classic IDE services.
