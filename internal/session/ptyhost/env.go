package ptyhost

import "strings"

// identityPrefixes name the variables that tie a process to the terminal or
// agent session it runs in. booster's sessions are their own, so a variable
// the core got from the shell or the agent session that started it must not
// reach them: a Claude Code that sees CLAUDECODE refuses to start, and a
// HERDR_PANE_ID would make Herdr tooling report into the wrong pane, and a
// BOOSTER_INSTANCE would put a booster that an agent builds into the core's
// own instance. Settings (for example CLAUDE_CODE_IDLE_COMPACT_MIN_TOKENS)
// stay.
var identityPrefixes = []string{
	"CLAUDECODE=",
	"CLAUDE_PID=",
	"CLAUDE_EFFORT=",
	"CLAUDE_CODE_SESSION_",
	"CLAUDE_CODE_CHILD_",
	"CLAUDE_CODE_MESSAGING_",
	"CLAUDE_CODE_ENTRYPOINT=",
	"CLAUDE_CODE_EXECPATH=",
	"HERDR_",
	"BOOSTER_INSTANCE=",
	"TERM=",
	"TERM_PROGRAM=",
	"TERM_PROGRAM_VERSION=",
	"COLORTERM=",
}

// terminalEnv describes the terminal that booster's sessions run in.
var terminalEnv = []string{
	"TERM=xterm-256color",
	"COLORTERM=truecolor",
	"TERM_PROGRAM=booster",
}

// childEnv is the environment of a session's program: base without the
// identity variables, plus booster's terminal.
func childEnv(base []string) []string {
	out := make([]string, 0, len(base)+len(terminalEnv))
	for _, kv := range base {
		if !hasIdentityPrefix(kv) {
			out = append(out, kv)
		}
	}
	return append(out, terminalEnv...)
}

func hasIdentityPrefix(kv string) bool {
	for _, p := range identityPrefixes {
		if strings.HasPrefix(kv, p) {
			return true
		}
	}
	return false
}
