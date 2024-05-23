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

	if cmd.reader == nil {
		if err = cfgReader(cmd, cCtx); err != nil {
			return err
		}
	}

	if cmd.cborCodec, err = massifs.NewRootSignerCodec(); err != nil {
		return err
	}
	cmd.rootReader = massifs.NewSignedRootReader(cmd.log, cmd.reader, cmd.cborCodec)
	return nil
}
