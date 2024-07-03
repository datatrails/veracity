package veracity

import (
	"context"

	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

type LocalMassifReader struct {
	log  logger.Logger
	opts massifs.MassifReaderOptions
}

func NewLocalMassifReader(
	log logger.Logger, opts ...massifs.MassifReaderOption,
) LocalMassifReader {
	r := LocalMassifReader{
		log: log,
	}
	for _, o := range opts {
		o(&r.opts)
	}
	return r
}

func (mr *LocalMassifReader) GetMassif(
	ctx context.Context, tenantIdentity string, massifIndex uint64,
	opts ...azblob.Option,
) (massifs.MassifContext, error) {

	var err error
	mc := massifs.MassifContext{
		TenantIdentity: tenantIdentity,
		LogBlobContext: massifs.LogBlobContext{
			BlobPath: massifs.TenantMassifBlobPath(tenantIdentity, massifIndex),
		},
	}
	if err = mr.readAndPrepareContext(ctx, &mc, opts...); err != nil {
		return massifs.MassifContext{}, err
	}
	return mc, nil
}

func (mr *LocalMassifReader) readAndPrepareContext(ctx context.Context, mc *massifs.MassifContext, opts ...azblob.Option) error {

	// just populates the mc.Data and last read etc -
	// so just need to read the appropriate blob here
	err := mc.ReadData(ctx, mr.store, opts...)
	if err != nil {
		return err
	}

	err = mc.Start.UnmarshalBinary(mc.Data)
	if err != nil {
		return err
	}
	if !mr.opts.noGetRootSupport {
		if err = mc.CreatePeakStackMap(); err != nil {
			return err
		}
	}
	return nil
}

func (mr *LocalMassifReader) GetHeadMassif(
	ctx context.Context, tenantIdentity string,
	opts ...azblob.Option,
) (massifs.MassifContext, error) {

	// tenant stuff is laregely irrelevant here
	var err error
	blobPrefixPath := TenantMassifPrefix(tenantIdentity)

	mc := massifs.MassifContext{
		TenantIdentity: tenantIdentity,
	}
	var massifCount uint64
	// just need to check the map for lates massif
	mc.LogBlobContext, massifCount, err = LastPrefixedBlob(ctx, mr.store, blobPrefixPath)
	if err != nil {
		return massifs.MassifContext{}, err
	}
	if massifCount == 0 {
		return massifs.MassifContext{}, massifs.ErrMassifNotFound
	}
	if err = mr.readAndPrepareContext(ctx, &mc, opts...); err != nil {
		return massifs.MassifContext{}, err
	}

	return mc, nil
}

// GetLazyContext reads the blob metadata of a logical blob but does _not_ read the data.
func (mr *LocalMassifReader) GetLazyContext(
	ctx context.Context, tenantIdentity string, which LogicalBlob,
	opts ...azblob.Option,
) (massifs.LogBlobContext, uint64, error) {

	// this is not very useful in our context this will become
	// not implemented for local blobs I think
	blobPrefixPath := TenantMassifPrefix(tenantIdentity)

	var massifIndex uint64

	var err error
	var logBlobContext massifs.LogBlobContext
	switch which {
	case FirstBlob:
		logBlobContext, err = FirstPrefixedBlob(ctx, mr.store, blobPrefixPath, opts...)
	case LastBlob:
		logBlobContext, massifIndex, err = LastPrefixedBlob(ctx, mr.store, blobPrefixPath, opts...)
	}
	if err != nil {
		return massifs.LogBlobContext{}, 0, err
	}
	return logBlobContext, massifIndex, nil
}

func (mr *LocalMassifReader) GetFirstMassif(
	ctx context.Context, tenantIdentity string,
	opts ...azblob.Option,
) (massifs.MassifContext, error) {

	// just get the first logfile
	var err error
	blobPrefixPath := TenantMassifPrefix(tenantIdentity)

	mc := massifs.MassifContext{
		TenantIdentity: tenantIdentity,
	}
	mc.LogBlobContext, err = FirstPrefixedBlob(ctx, mr.store, blobPrefixPath)
	if err != nil {
		return massifs.MassifContext{}, err
	}
	if err = mr.readAndPrepareContext(ctx, &mc, opts...); err != nil {
		return massifs.MassifContext{}, err
	}

	return mc, nil
}
