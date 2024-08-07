package veracity

import (
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/urfave/cli/v2"
)

func cfgRootReader(cmd *CmdCtx, cCtx *cli.Context) error {
	var err error
	if cmd.log == nil {
		if err = cfgLogging(cmd, cCtx); err != nil {
			return err
		}
	}

	if cmd.cborCodec, err = massifs.NewRootSignerCodec(); err != nil {
		return err
	}

	// TODO: We'll probably want to update this in future, in order to have a helpful default route.
	reader, err := cfgReader(cmd, cCtx, false)
	if err != nil {
		return err
	}

	cmd.rootReader = massifs.NewSignedRootReader(cmd.log, reader, cmd.cborCodec)
	return nil
}
