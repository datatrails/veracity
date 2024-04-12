package veracity

import (
	"context"
	"fmt"

	"github.com/datatrails/forestrie/go-forestrie/mmrblobs"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/urfave/cli/v2"
)

const (
	defaultMassifHeight = uint8(14)
)

// cfgMassifReader establishes the blob read only data accessor
// only azure blob storage is supported. Both emulated and produciton.
func cfgMassifReader(cmd *CmdCtx, cCtx *cli.Context) error {

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

	massifReader := mmrblobs.NewMassifReader(logger.Sugar, cmd.reader)
	cmd.massifReader = massifReader
	cmd.massifHeight = uint8(cCtx.Uint("height"))
	if cmd.massifHeight == 0 {
		cmd.massifHeight = defaultMassifHeight
	}

	return nil
}

// cfgMassif configures a massif reader and reads a massif
func cfgMassif(cmd *CmdCtx, cCtx *cli.Context) error {
	var err error

	if err = cfgMassifReader(cmd, cCtx); err != nil {
		return err
	}

	tenant := cCtx.String("tenant")
	if tenant == "" {
		return fmt.Errorf("tenant must be provided for this command")
	}

	ctx := context.Background()

	mmrIndex := cCtx.Uint64("mmrindex")
	massifIndex := cCtx.Uint64("massif")

	// mmrIndex zero is always going to be massifIndex 0 so we treat this the
	// same as though the massif option had been supplied as 0
	if massifIndex == uint64(0) && mmrIndex == uint64(0) {
		cmd.massif, err = cmd.massifReader.GetMassif(context.Background(), tenant, massifIndex)
		return err
	}

	// now, if we have a non zero mmrIndex, use it to (re)compute the massifIndex
	if mmrIndex > uint64(0) {
		massifIndex, err = mmrblobs.MassifIndexFromMMRIndex(cmd.massifHeight, mmrIndex)
		if err != nil {
			return err
		}

		cmd.massif, err = cmd.massifReader.GetMassif(context.Background(), tenant, massifIndex)
		return err
	}

	// If massifIndex is not provided it will be zero here, and that is a good
	// default.
	massif, err := cmd.massifReader.GetMassif(ctx, tenant, massifIndex)
	if err != nil {
		return err
	}
	cmd.massif = massif
	return nil
}
