package cli

import (
	"fmt"

	"github.com/sichengchen/eipgam/internal/provider"
	"github.com/sichengchen/eipgam/internal/settings"
)

// accountsDiscoveryCmd is `eipgam accounts discovery [on|off]` (#1535):
// whether eipgam reads the agents' own sign-ins (their credential files,
// the Keychain) and serves them as subscriptions; Settings → Privacy's
// Subscription accounts. With no argument it says which is in effect.
func accountsDiscoveryCmd(args []string) error {
	if len(args) > 1 || len(args) == 1 && args[0] != "on" && args[0] != "off" {
		return fmt.Errorf("usage: eipgam accounts discovery [on|off]")
	}
	if len(args) == 1 {
		s := settings.Load()
		s.NoAccountDiscovery = args[0] == "off"
		if err := settings.Save(s); err != nil {
			return err
		}
		provider.ForgetAccounts()
	}
	switch {
	case !settings.AccountDiscoveryOff():
		fmt.Println("account discovery is on: eipgam reads your agents' sign-ins and serves them as subscriptions")
	case settings.Shared().NoAccountDiscovery:
		fmt.Println("account discovery is off: eipgam reads none of your agents' sign-ins and serves no subscription; API-key providers work as before")
	default:
		fmt.Println("account discovery is off for this run: EIPGAM_NO_ACCOUNTS is set (saved: on)")
	}
	return nil
}
