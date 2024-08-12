package veracity

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// NewVerifyConsistencyCmd verifies consistency of an updated log against trusted copy of an earlier state.
//
//nolint:gocognit
func NewVerifyConsistencyCmd() *cli.Command {
	return &cli.Command{
		Name:    "verify-consistency",
		Aliases: []string{"consistent"},
		Usage:   `verify the consistency of a remote log update against a trusted copy of an earlier merkle log state.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: skipUncommittedFlagName, Value: false},
		},
		Action: func(cCtx *cli.Context) error {
			cmd := &CmdCtx{}

			var err error

			if err = cfgLogging(cmd, cCtx); err != nil {
				return err
			}

			return fmt.Errorf("not implemented")
		},
	}
}
