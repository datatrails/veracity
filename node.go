package veracity

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// NewNodeCmd prints out the identified mmr node
func NewNodeCmd() *cli.Command {
	return &cli.Command{Name: "node",
		Usage: "read a merklelog node",
		Flags: []cli.Flag{
			&cli.Int64Flag{
				Name: "mmrindex", Aliases: []string{"i"},
			},
		},
		Action: func(cCtx *cli.Context) error {
			var err error
			cmd := &CmdCtx{}
			err = cfgMassif(cmd, cCtx)
			if err != nil {
				return err
			}

			mmrIndex := cCtx.Uint64("mmrindex")

			value, err := cmd.massif.Get(mmrIndex)
			if err != nil {
				return err
			}
			fmt.Printf("%x\n", value)
			return nil
		},
	}
}
