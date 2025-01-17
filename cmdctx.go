package veracity

import (
	"context"

	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

// MassifGetter gets a specific massif based on the massifIndex given for a tenant log
type MassifGetter interface {
	GetMassif(
		ctx context.Context, tenantIdentity string, massifIndex uint64, opts ...massifs.ReaderOption,
	) (massifs.MassifContext, error)
}

type MassifReader interface {
	GetVerifiedContext(
		ctx context.Context, tenantIdentity string, massifIndex uint64,
		opts ...massifs.ReaderOption,
	) (*massifs.VerifiedContext, error)

	GetFirstMassif(
		ctx context.Context, tenantIdentity string, opts ...massifs.ReaderOption,
	) (massifs.MassifContext, error)
	GetHeadMassif(
		ctx context.Context, tenantIdentity string, opts ...massifs.ReaderOption,
	) (massifs.MassifContext, error)
	GetLazyContext(
		ctx context.Context, tenantIdentity string, which massifs.LogicalBlob, opts ...massifs.ReaderOption,
	) (massifs.LogBlobContext, uint64, error)
	MassifGetter
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

// Clone returns a copy of the CmdCtx with only those members that are safe to share copied.
// Those are:
//   - log - the result of cfgLogging
//
// All other members need to be initialzed by the caller if they are required in
// a specific go routine context.
func (c *CmdCtx) Clone() *CmdCtx {
	return &CmdCtx{log: c.log}
}
