package veracity

import (
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"github.com/urfave/cli/v2"
)

// cfgIDState initialises an idTimestamp generator
// of enabled workarounds.
func cfgIDState(cmd *CmdCtx, cCtx *cli.Context) error {
	var err error
	cmd.commitmentEpoch = 1 // the default, and correct until the unix epoch changes
	if cCtx.IsSet("commitment-epoch") {
		cmd.commitmentEpoch = uint8(cCtx.Uint64("commitment-epoch"))
	}

	cmd.idState, err = snowflakeid.NewIDState(snowflakeid.Config{
		CommitmentEpoch: cmd.commitmentEpoch,
		// There is no reason to override these for local use.
		WorkerCIDR: "0.0.0.0/16",
		PodIP:      "10.0.0.127",
	})

	return err
}
