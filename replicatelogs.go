package veracity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/urfave/cli/v2"
)

var (
	ErrChangesFlagIsExclusive         = errors.New("use --changes Or --massif and --tenant, not both")
	ErrNewReplicaNotEmpty             = errors.New("the local directory for a new replica already exists")
	ErrSealNotFound                   = errors.New("seal not found")
	ErrSealVerifyFailed               = errors.New("the seal signature verification failed")
	ErrFailedCheckingConsistencyProof = errors.New("failed to check a consistency proof")
	ErrFailedToCreateReplicaDir       = errors.New("failed to create a directory needed for local replication")
	ErrRequiredOption                 = errors.New("a required option was not provided")
	ErrRemoteLogTruncated             = errors.New("the local replica indicates the remote log has been truncated")
	ErrRemoteLogInconsistentRootState = errors.New("the local replica root state disagrees with the remote")
)

// NewReplicateLogsCmd updates a local replica of a remote log, verifying the mutual consistency of the two before making any changes.
//
//nolint:gocognit
func NewReplicateLogsCmd() *cli.Command {
	return &cli.Command{
		Name:    "replicate-logs",
		Aliases: []string{"consistent"},
		Usage:   `verifies the remote log and replicates it locally, ensuring the remote changes are consistent with the trusted local replica.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: skipUncommittedFlagName, Value: false},
			&cli.IntFlag{
				Name: "massif", Aliases: []string{"m"},
			},
			&cli.UintFlag{
				Usage: `The number of massif 'ancestors' to retain in the local replica.
This many massifs, prior to the requested, will be verified and retained localy.
If more exist locally, they are not removed (or reverified). If set to 0, a full replica is requested.`,
				Value: 0,
				Name:  "ancestors", Aliases: []string{"a"},
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
			&cli.StringFlag{
				Name: "changes",
				Usage: `
provide the path to a file enumerating the tenant massifs with changes you want
to verify and replicate.  This is mutually exclusive with the --massif and
--tenant flags. If none of --massif, --tenant or --changes are provided, the
changes are read from standard input.`,
			},
		},
		Action: func(cCtx *cli.Context) error {
			cmd := &CmdCtx{}

			// note: we don't use cfgMassifReader here because it does not
			// support setting replicaDir for the local reader, and infact we
			// need to configure both a local and a remote reader.

			var err error
			// The loggin configuration is safe to share accross go routines.
			if err = cfgLogging(cmd, cCtx); err != nil {
				return err
			}

			changes, err := readTenantMassifChanges(cCtx)
			if err != nil {
				return err
			}

			var wg sync.WaitGroup
			errChan := make(chan error, len(changes)) // buffered so it doesn't block

			for _, change := range changes {
				wg.Add(1)
				go func(change TenantMassif, errChan chan<- error) {
					defer wg.Done()
					replicator, err := NewVerifiedReplica(
						cCtx, cmd.Clone())
					if err != nil {
						errChan <- err
						return
					}
					err = replicator.ReplicateVerifiedUpdates(
						context.Background(),
						change.Tenant, uint32(change.Massif), uint32(cCtx.Uint("ancestors")),
					)
					if err != nil {
						errChan <- err
					}
				}(change, errChan)
			}

			// the error channel is buffered enough for each tenant, so this will not get deadlocked
			wg.Wait()
			close(errChan)

			var errs []error
			for err := range errChan {
				cmd.log.Infof(err.Error())
				errs = append(errs, err)
			}
			if len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	}
}

func readTenantMassifChanges(cCtx *cli.Context) ([]TenantMassif, error) {
	changesFile := cCtx.String("changes")
	tenantIdentity := cCtx.String("tenant")
	changedMassifIndex := cCtx.Int("massif")
	// Note: we could use GetHeadMassif to provide a default for --massif. But
	// that issues a list blobs query, and those are 10x more expensive. We have
	// aranged it so that verify-consistency does not issue *any* list blobs,
	// and so can reasonably be run in parallel. The watch command provides the
	// latest massif index, and the output of the watch command is the expected
	// source of the options to this command.

	var err error
	var changes []TenantMassif

	if changesFile != "" {
		if tenantIdentity != "" || changedMassifIndex != 0 {
			return nil, ErrChangesFlagIsExclusive
		}
		return filePathToTenantMassifs(changesFile)
	}

	changes = []TenantMassif{{Tenant: tenantIdentity, Massif: changedMassifIndex}}
	if tenantIdentity == "" && changedMassifIndex == 0 {
		changes, err = stdinToDecodedTenantMassifs()
		if err != nil {
			return nil, err
		}
	}
	return changes, nil
}

type VerifiedContextReader interface {
	massifs.VerifiedContextReader
}

type VerifiedReplica struct {
	log          logger.Logger
	writeOpener  massifs.WriteAppendOpener
	localReader  massifs.ReplicaReader
	remoteReader MassifReader
	cborCodec    cbor.CBORCodec
}

func NewVerifiedReplica(
	cCtx *cli.Context, cmd *CmdCtx,
) (*VerifiedReplica, error) {

	dataUrl := cCtx.String("data-url")
	reader, err := cfgReader(cmd, cCtx, dataUrl == "")
	if err != nil {
		return nil, err
	}
	if err = cfgRootReader(cmd, cCtx); err != nil {
		return nil, err
	}

	massifHeight := cCtx.Int64("height")
	if massifHeight > 255 {
		return nil, fmt.Errorf("massif height must be less than 256")
	}

	cache, err := massifs.NewLogDirCache(logger.Sugar, NewFileOpener())
	if err != nil {
		return nil, err
	}
	localReader, err := massifs.NewLocalReader(logger.Sugar, cache)
	if err != nil {
		return nil, err
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
			return nil, err
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

	return &VerifiedReplica{
		log:          logger.Sugar,
		writeOpener:  NewFileWriteOpener(),
		localReader:  &localReader,
		remoteReader: &remoteReader,
		cborCodec:    cmd.cborCodec,
	}, nil
}

// ReplicateVerifiedUpdates confirms that any additions to the remote log are
// consistent with the local replica Only the most recent local massif and seal
// need be retained for verification purposes.  If independent, off line,
// verification of inclusion is desired, retain as much of the log as is
// interesting.
func (v *VerifiedReplica) ReplicateVerifiedUpdates(
	ctx context.Context,
	tenantIdentity string, headMassifIndex uint32, ancestorCount uint32) error {

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

	// Read the most recently verified state from the lastLocal store. The
	// verification ensures the lastLocal replica has not been corrupted, but this
	// check trusts the seal stored locally with the head massif
	lastLocal, err := v.localReader.GetHeadVerifiedContext(ctx, tenantIdentity)
	if !isNilOrNotFound(err) {
		return err
	}

	var local *massifs.VerifiedContext

	// We always verify up to the requested massif.
	// ancestorCount is used to ensure there is a minimum number of verified massifs replicated locally.
	// Our verification always reads the remote massifs starting from requested - ancestorCount.
	// In the loop below we ensure three key things:
	// 1. If there is a local replica of the remote, we ensure the remote is
	//   consistent with the replica.
	// 2. If the remote starts a new massif, and we locally have its
	//    predecessor, we ensure the remote is consistent with the local.
	// 3. If there is no local replica, we create one from the remote.
	//
	// In call cases we first verify the remote against the remote seal.  The
	// --ancestors can be used to limit the number of predecesor massifs that
	// are verified before the changed massifs. In the case, consistency with
	// the local replica is only checked if the local replica is sufficiently up
	// to date. If it is not, no attempt is made to "fill the gap".

	i := headMassifIndex - ancestorCount
	if ancestorCount == 0 {
		// a full replica is requested
		i = 0
	}
	if lastLocal != nil {

		// In the case where the requested head is 2 or more ahead of the local
		// state we do not attempt to automaticaly back fill. This is so it is
		// possible to ensure a bounded number of requests (and copies) in
		// situations where verifiers have been off line for a long time.
		//
		// Explicit back filling is always possible by running with --ancestor=0
		// or by running the command multiple times with increasing --massif

		// With that accounted for, we then want to avoid re-verifying the state
		// we have already verified locally, hence the use of `max`.
		// We make no attempt to deal with "sparse" collections of massifs, the
		// most recent locally available is what we verify the remote against.

		i = max(lastLocal.Start.MassifIndex, i)
		if i > lastLocal.Start.MassifIndex+1 {
			// if the start of the ancestors is more than one massif ahead of
			// the local, then we start afresh.
			lastLocal = nil
		}
	}

	for ; i <= headMassifIndex; i++ {

		// Read the remote massif, providing our locally trusted (and just
		// re-verified) state as the trusted base This will cause the remote
		// massif to be verified against the remote seal and the seal we have
		// locally replicated. On the first iteration, lastLocal will be nil or
		// it will corespond to the remote masssif i or i-1.
		remote, err := v.remoteReader.GetVerifiedContext(
			ctx, tenantIdentity, uint64(i),
			append(remoteOptionsFromLocal(lastLocal), massifs.WithCBORCodec(v.cborCodec))...)
		if err != nil {
			// both the remote massif and it's seal must be present for the
			// verification to succeed, so we don't filter using isBlobNotFound
			// here.
			return err
		}

		// read the local massif, if it exists
		local, err = v.localReader.GetVerifiedContext(ctx, tenantIdentity, uint64(i))
		if !isNilOrNotFound(err) {
			return err
		}

		// copy the remote locally, safely replacing the coresponding local if one exists
		err = v.replicateVerifiedContext(ctx, local, remote)
		if err != nil {
			return err
		}

		// next round, use the just replicated remote as the trusted base for verification
		lastLocal = local
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
func (v *VerifiedReplica) replicateVerifiedContext(
	ctx context.Context,
	local *massifs.VerifiedContext, remote *massifs.VerifiedContext) error {

	if local == nil {
		return v.localReader.ReplaceVerifiedContext(remote, v.writeOpener)
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

	return v.localReader.ReplaceVerifiedContext(remote, v.writeOpener)
}
