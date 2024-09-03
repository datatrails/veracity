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

	forceProdUrl := cCtx.String("data-local") == "" && cCtx.String("data-url") == ""
	reader, err := cfgReader(cmd, cCtx, forceProdUrl)
	if err != nil {
		return err
	}

	cmd.rootReader = massifs.NewSignedRootReader(cmd.log, reader, cmd.cborCodec)
	return nil
}
