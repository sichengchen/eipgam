package cli

import (
	"fmt"
	"runtime"

	"github.com/sichengchen/eipgam/internal/autostart"
)

// eipgam autostart [on|off]: whether eipgam opens at login, in the tray.
func autostartCmd(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: eipgam autostart [on|off]")
	}
	if len(args) == 1 {
		switch args[0] {
		case "on", "off":
			if err := autostart.Set(args[0] == "on"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("usage: eipgam autostart [on|off]")
		}
	}
	switch autostart.Enabled() {
	case autostart.NeedsAllowing:
		// the record is there, and Set(true) can't undo the user's switch
		fmt.Println("eipgam is set to open at login, but it is switched off in System Settings › General › Login Items › Allow in the Background: switch it on there")
	case autostart.On:
		if runtime.GOOS == "android" {
			fmt.Println(green.Render("✓"), "eipgam starts the gateway at boot (requires Termux:Boot)")
		} else {
			fmt.Println(green.Render("✓"), "eipgam opens at login, in the tray")
		}
	default:
		fmt.Println("eipgam doesn't open at login")
	}
	return nil
}
