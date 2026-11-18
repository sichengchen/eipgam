package plugin

import (
	"context"
	"testing"
	"time"
)

// A community plugin npm has never had (CodeArts and River were merged and
// packed onto the dist branch, never published to npm) is offered the
// branch's newer version once the branch gains one: the hourly update
// installs it, Check for updates says "update", and Update installs it
// (star21321321124 on Discord: CodeArts 0.2.3 "auto-updated", every plugin
// "up to date", Update doing nothing, with 0.2.4 on the branch). A plugin
// with no newer version anywhere stays current, not unknown.
func TestDistOnlyPluginUpdates(t *testing.T) {
	for _, how := range []string{"hourly", "update"} {
		t.Run(how, func(t *testing.T) {
			sandbox(t)
			const pkg = "@magpie-community/opencode-codearts-auth"
			const river = "@magpie-community/opencode-river-auth"
			npmHasNothing(t)
			fakeDist(t, map[string][]string{pkg: {"0.2.3"}, river: {"0.1.0"}}, false)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			for _, p := range []string{pkg, river} {
				if _, err := Add(ctx, p); err != nil {
					t.Fatalf("Add %s: %v", p, err)
				}
			}
			if v := Installed(pkg); v != "0.2.3" {
				t.Fatalf("installed %q, want 0.2.3", v)
			}

			// the branch gains 0.2.4
			fakeDist(t, map[string][]string{pkg: {"0.2.3", "0.2.4"}, river: {"0.1.0"}}, false)

			checks := CheckNow(ctx)
			got := map[string]VersionCheck{}
			for _, c := range checks.Plugins {
				got[c.Package] = c
			}
			if c := got[pkg]; c.Status != "update" || c.Latest != "0.2.4" || c.Version != "0.2.3" {
				t.Fatalf("Check for updates found %+v for CodeArts, want 0.2.3 with an update to 0.2.4", c)
			}
			if c := got[river]; c.Status != "current" || c.Latest != "0.1.0" {
				t.Fatalf("Check for updates found %+v for River, want 0.1.0 current", c)
			}

			if how == "hourly" {
				u, err := CheckUpdates(ctx)
				if err != nil {
					t.Fatalf("CheckUpdates: %v", err)
				}
				if len(u.Updated) != 1 || u.Updated[0].Package != pkg || u.Updated[0].From != "0.2.3" || u.Updated[0].To != "0.2.4" {
					t.Fatalf("the hourly look updated %+v, want CodeArts 0.2.3 to 0.2.4", u.Updated)
				}
			} else if err := Upgrade(ctx, pkg); err != nil {
				t.Fatalf("Update: %v", err)
			}
			if v := Installed(pkg); v != "0.2.4" {
				t.Fatalf("installed %q, want 0.2.4", v)
			}
			if v := Installed(river); v != "0.1.0" {
				t.Fatalf("River is at %q, want 0.1.0", v)
			}
		})
	}
}
