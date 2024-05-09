package veracity

import (
	"github.com/datatrails/forestrie/go-forestrie/massifs"
	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
)

// CmdCtx holds shared config and config derived state for all commands
type CmdCtx struct {
	log logger.Logger
	// storer *azblob.Storer
	reader       azblob.Reader
	massifReader massifs.MassifReader
	massif       massifs.MassifContext

	massifHeight uint8

	bugs map[string]bool
}
