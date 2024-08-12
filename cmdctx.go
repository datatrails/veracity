package veracity

import (
	"context"

	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

type MassifReader interface {
	GetFirstMassif(
		ctx context.Context, tenantIdentity string, opts ...massifs.ReaderOption,
	) (massifs.MassifContext, error)
	GetHeadMassif(
		ctx context.Context, tenantIdentity string, opts ...massifs.ReaderOption,
	) (massifs.MassifContext, error)
	GetLazyContext(
		ctx context.Context, tenantIdentity string, which massifs.LogicalBlob, opts ...massifs.ReaderOption,
	) (massifs.LogBlobContext, uint64, error)
	GetMassif(
		ctx context.Context, tenantIdentity string, massifIndex uint64, opts ...massifs.ReaderOption,
	) (massifs.MassifContext, error)
}

// CmdCtx holds shared config and config derived state for all commands
type CmdCtx struct {
	log logger.Logger
	// storer *azblob.Storer
	//reader       azblob.Reader
	massifReader MassifReader
	readerURL    string
	cborCodec    cbor.CBORCodec
	rootReader   massifs.SignedRootReader
	massif       massifs.MassifContext

	massifHeight uint8

	bugs map[string]bool
}
