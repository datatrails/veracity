// Package localmassifs provides functionality to read and verify massifs from a local filesystem.
package localmassifs

import (
	"context"
	"fmt"

	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

type ReaderOptions struct {
	log          logger.Logger
	MassifsDir   string
	SealsDir     string
	MassifHeight uint8
	CBORCodec    cbor.CBORCodec
}

func NewReaderDefaultConfig(log logger.Logger, massifsDir, sealsDir string) (ReaderOptions, error) {

	var err error
	options := ReaderOptions{
		MassifsDir:   massifsDir,
		SealsDir:     sealsDir,
		MassifHeight: 14,
	}
	if options.CBORCodec, err = massifs.NewRootSignerCodec(); err != nil {
		return ReaderOptions{}, err
	}
	return options, nil
}

// ReadVerifiedHeadMassif reads the verified head massif from the specified directory
func ReadVerifiedHeadMassif(options ReaderOptions, baseOpts ...massifs.DirCacheOption) (*massifs.VerifiedContext, error) {

	var err error

	opts := []massifs.DirCacheOption{
		massifs.WithDirCacheMassifLister(NewSuffixDirLister(".log")),
		massifs.WithDirCacheSealLister(NewSuffixDirLister(".sth")),
		massifs.WithReaderOption(massifs.WithMassifHeight(options.MassifHeight)),
		massifs.WithReaderOption(massifs.WithCBORCodec(options.CBORCodec)),
	}
	opts = append(opts, baseOpts...)

	cache, err := massifs.NewLogDirCache(options.log, NewFileOpener(), opts...)
	if err != nil {
		return nil, err
	}

	reader, err := massifs.NewLocalReader(options.log, cache)
	if err != nil {
		return nil, err
	}

	cache.ReplaceOptions(opts...)

	massifCache, err := cache.ReadMassifDirEntry(options.MassifsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read massif dir entry: %w", err)
	}
	massifInfo := massifCache.GetInfo()
	fmt.Printf("Read massif %d to %d from %s\n", massifInfo.FirstMassifIndex, massifInfo.HeadMassifIndex, massifInfo.Directory)

	var sealCache massifs.DirCacheEntry

	if options.MassifsDir == options.SealsDir {
		sealCache = massifCache
		err = cache.FindSealFiles(options.MassifsDir)
	} else {
		sealCache, err = cache.ReadSealDirEntry(options.SealsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read massif dir entry: %w", err)
		}
	}

	sealInfo := sealCache.GetInfo()
	fmt.Printf("Read seals for massifs %d to %d from %s\n", sealInfo.FirstSealIndex, sealInfo.HeadSealIndex, sealInfo.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read massif dir entry: %w", err)
	}

	mc, err := cache.ReadMassif(massifInfo.Directory, uint64(massifInfo.HeadMassifIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to read massif %d from %s: %w", massifInfo.HeadMassifIndex, massifInfo.Directory, err)
	}

	seal, err := cache.ReadSeal(sealInfo.Directory, uint64(massifInfo.HeadMassifIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to read seal for massif %d from %s: %w", massifInfo.HeadMassifIndex, sealInfo.Directory, err)
	}
	verified, err := reader.VerifyContext(context.Background(), *mc, massifs.WithCheckpoint(&seal.Sign1Message, &seal.MMRState))
	if err != nil {
		return nil, fmt.Errorf("failed to read massif %d from %s: %w", massifInfo.HeadMassifIndex, massifInfo.Directory, err)
	}
	return verified, nil
}
