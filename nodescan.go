package veracity

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/datatrails/forestrie/go-forestrie/mmrblobs"
	"github.com/urfave/cli/v2"
)

// NewNodeScan implements a sub command which linearly scans for a node in a blob
// This is a debugging tool
func NewNodeScanCmd() *cli.Command {
	return &cli.Command{Name: "nodescan",
		Usage: "scan a log for a particular node value. this is a debugging tool",
		Flags: []cli.Flag{
			&cli.Int64Flag{
				Name: "massif", Aliases: []string{"m"},
			},
			&cli.StringFlag{
				Name: "value", Aliases: []string{"v"},
			},
			&cli.BoolFlag{Name: "massif-relative", Aliases: []string{"r"}},
		},
		Action: func(cCtx *cli.Context) error {
			var err error
			cmd := &CmdCtx{}

			if err = cfgMassif(cmd, cCtx); err != nil {
				return err
			}

			targetValue, err := hex.DecodeString(cCtx.String("value"))
			if err != nil {
				return err
			}
			start := cmd.massif.LogStart()
			count := cmd.massif.Count()
			for i := uint64(0); i < count; i++ {
				entry := cmd.massif.Data[start+i*mmrblobs.ValueBytes : start+i*mmrblobs.ValueBytes+mmrblobs.ValueBytes]
				if bytes.Compare(entry, targetValue) == 0 {
					fmt.Printf("%d\n", i+cmd.massif.Start.FirstIndex)
					return nil
				}
			}
			return fmt.Errorf("'%s' not found", cCtx.String("value"))
		},
	}
}
