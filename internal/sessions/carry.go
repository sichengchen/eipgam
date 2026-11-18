package sessions

// Carrying a session on in another agent (#845): only where that agent reads
// the session's own file as it is, so nothing is converted, and the file is
// left as it was.

// Claude Code's and Codex's sessions are converted for each other
// (carry_convert.go).

import (
	"runtime"
	"strings"
)

// Carry is another agent a session can be carried on in, and the shell
// line that does it in the session's folder. A Convert one has no line
// yet: the session is converted into a new one of that agent's when the
// user asks (CarryOver, carry_convert.go), and the line opens that.
type Carry struct {
	Agent   string `json:"agent"`
	Command string `json:"command,omitempty"`
	Convert bool   `json:"convert,omitempty"`
}

// carries are the other agents a session can be carried on in.
func carries(s Session) []Carry {
	if s.ReadOnly || s.WSL != "" || s.Path == "" || s.Cwd == "" {
		return nil
	}
	if to := convertsTo[s.Agent]; to != "" && agentThere(to) {
		return []Carry{{Agent: to, Convert: true}}
	}
	return nil
}

// inFolder is a shell line running run in the folder cwd.
func inFolder(cwd, run string) string {
	if runtime.GOOS == "windows" {
		return "Set-Location -LiteralPath " + argQuote(cwd) + "; " + run
	}
	return "cd " + argQuote(cwd) + " && " + run
}

// argQuote is s as one argument of this system's shell.
func argQuote(s string) string {
	if runtime.GOOS == "windows" {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return shellQuote(s)
}
