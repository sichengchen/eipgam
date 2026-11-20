package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sichengchen/eipgam/internal/gateway"
)

// status is what `eipgam status` tells of the eipgam serving the gateway.
type status struct {
	Running bool   `json:"running"`
	URL     string `json:"url"`
	// Addr is where it listens (0.0.0.0:10100), told by the process itself
	// when it wrote it down, else the address asked
	Addr string `json:"addr"`
	// Role is how that process was started: app, tray, web, serve or tui
	Role    string `json:"role,omitempty"`
	Pid     int    `json:"pid,omitempty"`
	Version string `json:"version,omitempty"`
	Started string `json:"started,omitempty"`
	// File is where the running eipgam writes this down for a program that
	// reads files (a watchdog)
	File string `json:"file"`
}

// statusUsage is eipgam status --help.
const statusUsage = `usage: eipgam status [--json] [--addr host:port]
  says which eipgam serves the gateway on this computer: its role (app, tray, web,
  serve or tui), process id, version and listen address. eipgam.exe tray and
  eipgam.exe mcp image are one image name; the pid here is the one that serves.
  --json prints one JSON object (running false when none answers), for a watchdog.
  The running eipgam also keeps the same in gateway.json in eipgam's config folder
  while it serves. It looks where --addr says, else at EIPGAM_ADDR, else where the
  running eipgam wrote down that it listens, else at the port in Settings' Gateway
  address.`

func statusCmd(args []string) error {
	asJSON := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			asJSON = true
			continue
		}
		rest = append(rest, a)
	}
	base, err := clientBase("status", rest)
	if err != nil {
		return err
	}
	st := status{URL: base, File: gateway.ListeningPath()}
	st.Addr = base[len("http://"):]
	if o := gateway.ServedAt(base); o.Running {
		st.Running, st.Role, st.Pid, st.Version = true, o.Serves, o.Pid, o.Version
		if l, ok := gateway.ReadListening(); ok && l.Pid == o.Pid {
			st.Addr, st.Started = l.Addr, l.Started
		}
	}
	if asJSON {
		b, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	if !st.Running {
		fmt.Println(amber.Render("○"), "no eipgam answers on", st.URL)
		return nil
	}
	line := "eipgam " + st.Version + " serves the gateway on " + bold.Render(st.Addr)
	if st.Role != "" {
		line += " · " + st.Role
	}
	if st.Pid > 0 {
		line += " · pid " + strconv.Itoa(st.Pid)
	}
	fmt.Println(green.Render("●"), line)
	return nil
}
