package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sichengchen/eipgam/internal/appdir"
	"github.com/sichengchen/eipgam/internal/codexrelay"
	"github.com/sichengchen/eipgam/internal/gateway"
	"github.com/sichengchen/eipgam/internal/update"
)

// eipgam data: where eipgam keeps its files, and moving them to another
// disk (CHENGJIAHEGOXIS and buhaodeshiyehao on X; appdir/move.go). A move
// is made at once when no eipgam serves the gateway, and otherwise at the
// next start of the app, which Settings › Sync's Data folder restarts
// it for.
func dataCmd(args []string) error {
	if len(args) == 0 || args[0] == "where" {
		p := appdir.Where()
		fmt.Println("files:", p.Config)
		if p.ConfigAt != "" {
			fmt.Println("       →", p.ConfigAt)
		}
		fmt.Println("cache:", p.Cache)
		if p.CacheAt != "" {
			fmt.Println("       →", p.CacheAt)
		}
		if p.Portable {
			fmt.Println("portable: they move with eipgam's data folder")
		}
		switch p.Pending {
		case "":
		case "home":
			fmt.Println("moving back to the default folders when eipgam next starts")
		default:
			fmt.Println("moving to", p.Pending, "when eipgam next starts")
		}
		if p.Error != "" {
			fmt.Println("the last move wasn't made:", p.Error)
		}
		return appdir.Unreachable()
	}
	switch args[0] {
	case "move":
		if len(args) != 2 || strings.HasPrefix(args[1], "-") && args[1] != "--home" {
			return errors.New("usage: eipgam data move <folder> | --home")
		}
		dir := args[1]
		if dir == "--home" {
			dir = ""
		} else if a, err := filepath.Abs(dir); err == nil {
			dir = a
		}
		if gatewayUp() {
			root, err := appdir.AskMove(dir)
			if err != nil {
				return err
			}
			fmt.Printf("eipgam is running: its files move to %s when it next starts (quit it and open it again)\n", homeOr(root))
			return nil
		}
		root, err := appdir.Target(dir)
		if err != nil {
			return err
		}
		appdir.CancelMove()
		if err := appdir.Move(root); err != nil {
			return err
		}
		fmt.Println(green.Render("●"), "eipgam's files are in", homeOr(root), "now")
		return nil
	case "cancel":
		return appdir.CancelMove()
	case "-h", "--help", "help":
		fmt.Print(dataUsage)
		return nil
	}
	return fmt.Errorf("unknown data command %q\n%s", args[0], dataUsage)
}

const dataUsage = `eipgam data                     where eipgam keeps its files and its cache
eipgam data move <folder>       move them into <folder>/eipgam (another disk), leaving a link where they were
eipgam data move --home         move them back
eipgam data cancel              forget a move asked for while eipgam was running
`

func homeOr(root string) string {
	if root == "" {
		return appdir.Where().Config
	}
	return root
}

// gatewayUp says whether a eipgam serves the gateway: one running holds
// files in the folders a move would take away.
var gatewayUp = func() bool {
	codexrelay.Leave() // what a quitting eipgam left at the port lets go of it
	return gateway.ServedAt(gateway.ClientURL()).Running
}

// movePending makes the move Settings asked for, as the app starts again
// for it: after the eipgam it replaces has exited, and before anything is
// read or written. With another eipgam still serving the gateway (eipgam
// serve, a launch agent) the move waits for a start without one.
func movePending() {
	if !appdir.MoveAsked() {
		return
	}
	update.AwaitPredecessor()
	if gatewayUp() {
		fmt.Fprintln(os.Stderr, "eipgam: its folders move when no other eipgam is running")
		return
	}
	if _, err := appdir.MovePending(); err != nil {
		fmt.Fprintln(os.Stderr, "eipgam: its folders weren't moved:", err)
	}
}

// unreachableStop ends eipgam when its folders were moved to a disk that
// isn't connected, saying so in a box for the app, which has no terminal.
// eipgam data still answers, to say where they are.
func unreachableStop(args []string) {
	err := appdir.Unreachable()
	if err == nil || len(args) > 0 && args[0] == "data" {
		return
	}
	fmt.Fprintln(os.Stderr, "eipgam:", err)
	if hasGUI && (len(args) == 0 || servesPages(args) && args[0] != "serve" && args[0] != "tui" && args[0] != "web") {
		alert("eipgam", err.Error())
	}
	os.Exit(1)
}
