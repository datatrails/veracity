package veracity

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/urfave/cli/v2"
)

var (
	ErrNewReplicaNotEmpty             = errors.New("the local directory for a new replica already exists")
	ErrSealNotFound                   = errors.New("seal not found")
	ErrSealVerifyFailed               = errors.New("the seal signature verification failed")
	ErrFailedCheckingConsistencyProof = errors.New("failed to check a consistency proof")
	ErrFailedToCreateReplicaDir       = errors.New("failed to create a directory needed for local replication")
	ErrRequiredOption                 = errors.New("a required option was not provided")
	ErrRemoteLogTruncated             = errors.New("the local replica indicates the remote log has been truncated")
	ErrRemoteLogInconsistentRootState = errors.New("the local replica root state disagrees with the remote")
)

// NewUpdateReplicaCmd updates a local replica of a remote log, verifying the mutual consistency of the two before making any changes.
//
//nolint:gocognit
func NewUpdateReplicaCmd() *cli.Command {
	return &cli.Command{
		Name:    "update-replica",
		Aliases: []string{"consistent"},
		Usage:   `updates a local replica of a remote log, verifying the mutual consistency of the two before making any changes.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: skipUncommittedFlagName, Value: false},
			&cli.IntFlag{
				Name: "massif", Aliases: []string{"m"},
			},
			&cli.StringFlag{
				Name: "replicadir",
				Usage: `the root directory for all tenant log replicas,
the structure under this directory mirrors the /verifiabledata/merklelogs paths
in the publicly accessible remote storage`,
				Aliases: []string{"d"},
				Value:   ".",
			},
			&cli.StringFlag{
				Name: "sealer-key",
				Usage: `to ensure  the remote seal is signed by the correct
key, set this to the public datatrails sealing key,
having obtained its value from a source you trust`,
				Aliases: []string{"pub"},
			},
		},
		Action: func(cCtx *cli.Context) error {
			cmd := &CmdCtx{}

			var err error

			if err = cfgLogging(cmd, cCtx); err != nil {
				return err
			}

			// note: we don't use cfgMassifReader here because it does not
			// support setting replicaDir for the local reader, and infact we
			// need to configure both a local and a remote reader.

			// It could make sense in future to support local <- local for re-verification purposes.

			tenantIdentity := cCtx.String("tenant")
			headMassifIndex := cCtx.Int("massif")
			dataUrl := cCtx.String("data-url")
			reader, err := cfgReader(cmd, cCtx, dataUrl == "")
			if err != nil {
				return err
			}
			cfgRootReader(cmd, cCtx)

			massifHeight := cCtx.Int64("height")
			if massifHeight > 255 {
				return fmt.Errorf("massif height must be less than 256")
			}

			cache, err := massifs.NewLogDirCache(logger.Sugar, NewFileOpener())
			if err != nil {
				return err
			}
			localReader, err := massifs.NewLocalReader(logger.Sugar, cache)
			if err != nil {
				return err
			}

			opts := []massifs.DirCacheOption{
				massifs.WithDirCacheReplicaDir(cCtx.String("replicadir")),
				massifs.WithDirCacheMassifLister(NewDirLister()),
				massifs.WithDirCacheSealLister(NewDirLister()),
				massifs.WithReaderOption(massifs.WithMassifHeight(uint8(massifHeight))),
				massifs.WithReaderOption(massifs.WithSealGetter(&localReader)),
				massifs.WithReaderOption(massifs.WithCBORCodec(cmd.cborCodec)),
			}

			// This will require that the remote seal is signed by the key
			// provided here. If it is not, even if the seal is valid, the
			// verification will fail with a suitable error.
			pemString := cCtx.String("sealer-key")
			if pemString != "" {
				pem, err := DecodeECDSAPublicString(pemString)
				if err != nil {
					return err
				}
				opts = append(opts, massifs.WithReaderOption(massifs.WithTrustedSealerPub(pem)))
			}

			// For the localreader, the seal getter is the local reader itself.
			// So we need to make use of ReplaceOptions on the cache, so we can
			// provide the options after we have created the local reader.
			cache.ReplaceOptions(opts...)

			remoteReader := massifs.NewMassifReader(
				logger.Sugar, reader,
				massifs.WithSealGetter(&cmd.rootReader),
			)

			v := &ConsistentReplica{
				log:          logger.Sugar,
				writeOpener:  NewFileWriteOpener(),
				localReader:  &localReader,
				remoteReader: &remoteReader,
				cborCodec:    cmd.cborCodec,
			}

			return v.ReplicateVerifiedUpdates(context.Background(), tenantIdentity, uint32(headMassifIndex))
		},
	}
}

type VerifiedContextReader interface {
	massifs.VerifiedContextReader
}

type ConsistentReplica struct {
	log          logger.Logger
	writeOpener  massifs.WriteAppendOpener
	localReader  massifs.ReplicaReader
	remoteReader MassifReader
	cborCodec    cbor.CBORCodec
}

// ReplicateVerifiedUpdates confirms that any additions to the remote log are
// consistent with the local replica Only the most recent local massif and seal
// need be retained for verification purposes.  If independent, off line,
// verification of inclusion is desired, retain as much of the log as is
// interesting.
func (v *ConsistentReplica) ReplicateVerifiedUpdates(
	ctx context.Context,
	tenantIdentity string, headMassifIndex uint32) error {

	isNilOrNotFound := func(err error) bool {
		if err == nil {
			return true
		}
		if errors.Is(err, massifs.ErrLogFileSealNotFound) {
			return true
		}
		if errors.Is(err, massifs.ErrLogFileMassifNotFound) {
			return true
		}
		return false
	}

	remoteOptionsFromLocal := func(local *massifs.VerifiedContext) []massifs.ReaderOption {
		var opts []massifs.ReaderOption
		if local == nil {
			return opts
		}
		return append(opts, massifs.WithTrustedBaseState(local.MMRState))
	}

	if err := v.localReader.EnsureReplicaDirs(tenantIdentity); err != nil {
		return err
	}

	// Read the most recently verified state from the local store. The
	// verification ensures the local replica has not been corrupted, but this
	// check trusts the seal stored locally with the head massif
	local, err := v.localReader.GetHeadVerifiedContext(ctx, tenantIdentity)
	if !isNilOrNotFound(err) {
		return err
	}

	// each time round the loop below we always read and verify the remote.
	// after each remote, we will always have a local as it will be the replica
	// of the last remote. on the first time round, we may be initializing an
	// empty replica, so in that case, we will not have a local and we will be
	// starting at the requested headMassifIndex.
	//
	// In all cases where we do have a local, even where we just read and
	// verified it on a previous iteration, we provide its state as the trusted
	// base for the next remote read.

	// if we don't yet have a local replica, we start at the requested head and just replicate the verified remote state.
	i := headMassifIndex
	if local != nil {
		i = local.Start.MassifIndex
	}

	for ; i <= headMassifIndex; i++ {

		// note: err is from the first local read above, or the local read at the end of the loop body

		// Read the remote massif corresponding to last local, providing our locally trusted (and just re-verified) state as the trusted base
		// This will cause the remote massif to be verified against the remote seal and the seal we have locally replicated.
		remote, err := v.remoteReader.GetVerifiedContext(
			ctx, tenantIdentity, uint64(i),
			append(remoteOptionsFromLocal(local), massifs.WithCBORCodec(v.cborCodec))...)
		if err != nil {
			return err
		}

		err = v.replicateVerifiedContext(ctx, local, remote)
		if err != nil {
			return err
		}

		local, err = v.localReader.GetVerifiedContext(ctx, tenantIdentity, uint64(i))
		if !isNilOrNotFound(err) {
			return err
		}
	}

	return nil
}

// replicateVerifiedContext is used to replicate a remote massif which may be an
// extension of a previously verified local copy.
//
// If local is nil, this method simply replicates the verified remote unconditionally.
//
// Otherwise, local and remote are required to be the same tenant and the same massif.
// This method then deals with ensuring the remote is a consistent extension of
// local before replacing the previously verified local.
//
// This method has no side effects in the case where the remote and the local
// are verified to be identical, the original local instance is retained.
func (v *ConsistentReplica) replicateVerifiedContext(
	ctx context.Context,
	local *massifs.VerifiedContext, remote *massifs.VerifiedContext) error {

	if local == nil {
		return v.localReader.ReplaceVerifiedContext(ctx, remote, v.writeOpener)
	}

	if local.TenantIdentity != remote.TenantIdentity {
		return fmt.Errorf("can't replace, tenant identies don't match: local %s vs remote %s", local.TenantIdentity, remote.TenantIdentity)
	}

	if local.Start.MassifIndex != remote.Start.MassifIndex {
		return fmt.Errorf(
			"can't replace, massif indices don't match: local %d vs remote %d",
			local.Start.MassifIndex, remote.Start.MassifIndex)
	}

	tenantIdentity := local.TenantIdentity
	massifIndex := local.Start.MassifIndex

	if len(local.Data) > len(remote.Data) {
		// the remote log has been truncated since we last looked
		return fmt.Errorf("%w: %s, massif=%d", ErrRemoteLogTruncated, tenantIdentity, massifIndex)
	}

	// if the remote and local are the same, we are done, provided the roots still match
	if len(local.Data) == len(remote.Data) {
		if !bytes.Equal(local.ConsistentRoots, remote.ConsistentRoots) {
			return fmt.Errorf("%w: %s, massif=%d", ErrRemoteLogInconsistentRootState, tenantIdentity, massifIndex)
		}
		return nil
	}

	return v.localReader.ReplaceVerifiedContext(ctx, remote, v.writeOpener)
}
