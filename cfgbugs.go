package veracity

import (
	"fmt"
	"slices"

	"github.com/urfave/cli/v2"
)

var (
	// recovers timestamp_committed from merklelog_entry.commit.idtimestamp prior to hashing
	Bug9308 = "9308"

	Bugs = []string{
		Bug9308,
	}
)

func IsSupportedBug(id string) bool {
	return slices.Contains(Bugs, id)
}

func Bug(cmd *CmdCtx, id string) bool {
	if cmd.bugs == nil {
		return false
	}
	return cmd.bugs[id]
}

// cfgBugs checks the requested bug workarounds are valid and populates the map
// of enabled workarounds.
func cfgBugs(cmd *CmdCtx, cCtx *cli.Context) error {
	cmd.bugs = map[string]bool{}

	// just one supported atm
	id := cCtx.String("bug")
	if id != "" {
		if !IsSupportedBug(id) {
			return fmt.Errorf("bug: %s no supported work around or accommodation", id)
		}
		cmd.bugs[id] = true
	}

	return nil
}
