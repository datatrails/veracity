package veracity

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// NewProveCmd (will) generate a proof and node path for the argument node
func NewProveCmd() *cli.Command {
	return &cli.Command{Name: "prove",
		Usage: "generate a proof for merklelog node",
		Action: func(cCtx *cli.Context) error {
			fmt.Println("prove: ", cCtx.Args().First())
			return nil
		},
	}
}
