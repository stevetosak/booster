# booster

booster is an IDE for the agentic workflow: it hosts coding agents in full
terminals beside a classic IDE, so the developer works with agents and stays
close to the code.

## Language

**Operator**:
The person who directs the agents and owns every decision.
_Avoid_: user (in design text), owner

**Project**:
A code base the Operator works on, opened by name from booster, not by folder; it may span a main checkout and its worktrees.
_Avoid_: folder, workspace, repo (when the unit of work is meant)

**Agent session**:
One running agent (Claude Code or pi) in its own PTY, owned by the session host and shown in an IDE terminal tab.
_Avoid_: pane, agent (when the running instance is meant)

**Core**:
The headless booster service that owns Projects, Agent sessions, Actions, agentic config, analytics and the Board; every UI talks to it.
_Avoid_: backend, server, daemon

**Host**:
The program that supplies the classic IDE side and shows booster's UI through an adapter over the Core.
_Avoid_: frontend, shell, editor

**Session host**:
The part of the Core that owns the PTYs, keeps each screen for reattach and injects input; it implements `SessionBackend`.
_Avoid_: multiplexer, terminal server

**Action**:
A named, repeatable agent request offered as an IDE feature: a prompt template, optional extra instructions from the Operator and the agent that runs it.
_Avoid_: macro, command, shortcut

**Board**:
The view of the agents' Tasks across every Project: what is done, what is open, what waits for the Operator.
_Avoid_: dashboard, kanban
