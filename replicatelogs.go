package veracity

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/massifs/watcher"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/gosuri/uiprogress"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/rand"
)

const (
	// baseDefaultRetryDelay is the base delay for retrying transient errors. A little jitter is added.
	// 429 errors which provide a valid Retry-After header will honor that header rather than use this.
	baseDefaultRetryDelay = 2 * time.Second
	defaultConcurrency    = 5

	// The default data retention policy is 2 years, so this is a generous default for "all data".
	tenYearsOfHours = 10 * 365 * 24 * time.Hour

	// jitterRangeMS is the range from 0 to jitter in milliseconds
	jitterRangeMS = 100

	// massifHeightMax is the maximum massif height
	massifHeightMax = 255
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
		Aliases: []string{"replicate"},
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
			&cli.BoolFlag{
				Name:    "progress",
				Usage:   `show progress of the replication process`,
				Value:   false,
				Aliases: []string{"p"},
			},
			&cli.BoolFlag{
				Name:  "latest",
				Usage: `find the latest changes automaticaly. When --latest is set, a list of tenants can be provided to --tenant to limit the tenant logs to be replicated.`,
				Value: false,
			},
			&cli.IntFlag{
				Name:    "retries",
				Aliases: []string{"r"},
				Value:   -1, // -1 means no limit
				Usage: `
Set a maximum number of retries for transient error conditions. Set 0 to disable retries.
By default transient errors are re-tried without limit, and if the error is 429, the Retry-After header is honored.`,
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Value:   defaultConcurrency,
				Aliases: []string{"c"},
				Usage: fmt.Sprintf(
					`The number of concurrent replication operations to run, defaults to %d. A high number is a sure way to get rate limited`, defaultConcurrency),
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

			// There isn't really a better context. We could implement user
			// defined timeouts for "lights out/ci" use cases in future. Humans can ctrl-c
			changes, err := readTenantMassifChanges(context.Background(), cCtx, cmd)
			if err != nil {
				return err
			}

			if cCtx.Bool("progress") {
				uiprogress.Start()
			}
			progress := newProgressor(cCtx, "tenants", len(changes))

			concurency := min(len(changes), max(1, cCtx.Int("concurrency")))
			for i := 0; i < len(changes); i += concurency {
				err = replicateChanges(cCtx, cmd, changes[i:min(i+concurency, len(changes))], progress)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}
}

// replicateChanges replicate the changes for the provided slice of tenants.
// Paralelism is limited by breaking the total changes into smaller slices and calling this function
func replicateChanges(cCtx *cli.Context, cmd *CmdCtx, changes []TenantMassif, progress Progresser) error {

	var wg sync.WaitGroup
	errChan := make(chan error, len(changes)) // buffered so it doesn't block

	for _, change := range changes {
		wg.Add(1)
		go func(change TenantMassif, errChan chan<- error) {
			defer wg.Done()
			defer progress.Completed()

			retries := max(-1, cCtx.Int("retries"))
			for {

				replicator, startMassif, endMassif, err := initReplication(cCtx, cmd, change)
				if err != nil {
					errChan <- err
					return
				}

				// There isn't really a better context. We could implement user
				// defined timeouts for "lights out/ci" use cases in future. Humans can ctrl-c
				err = replicator.ReplicateVerifiedUpdates(
					context.Background(),
					change.Tenant, startMassif, endMassif,
				)
				if err == nil {
					return
				}

				// 429 is the only transient error we currently re-try
				var retryDelay time.Duration
				retryDelay, ok := massifs.IsRateLimiting(err)
				if !ok || retries == 0 {
					// not transient
					errChan <- err
					return
				}
				if retryDelay == 0 {
					retryDelay = defaultRetryDelay(err)
				}

				// underflow will actually terminate the loop, but that would have been running for an infeasable amount of time
				retries--
				// in the default case, remaining is always reported as -1
				cmd.log.Infof("retrying in %s, remaining: %d", retryDelay, max(-1, retries))
			}
		}(change, errChan)
	}

	// the error channel is buffered enough for each tenant, so this will not get deadlocked
	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		cmd.log.Infof("%v", err)
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	if len(changes) == 1 {
		cmd.log.Infof("replication complete for tenant %s", changes[0].Tenant)
	} else {
		cmd.log.Infof("replication complete for %d tenants", len(changes))
	}
	return nil
}

func initReplication(cCtx *cli.Context, cmd *CmdCtx, change TenantMassif) (*VerifiedReplica, uint32, uint32, error) {

	replicator, err := NewVerifiedReplica(cCtx, cmd.Clone())
	if err != nil {
		return nil, 0, 0, err
	}
	endMassif := uint32(change.Massif)
	startMassif := uint32(0)
	if cCtx.IsSet("ancestors") && uint32(cCtx.Int("ancestors")) < endMassif {
		startMassif = endMassif - uint32(cCtx.Int("ancestors"))
	}
	return replicator, startMassif, endMassif, nil
}

func defaultRetryDelay(_ error) time.Duration {
	// give the delay some jitter, this is universally a good practice
	return baseDefaultRetryDelay + time.Duration(rand.Intn(jitterRangeMS))*time.Millisecond
}

func newProgressor(cCtx *cli.Context, barName string, increments int) Progresser {

	if !cCtx.Bool("progress") {
		return NewNoopProgress()
	}
	return NewStagedProgress(barName, increments)
}

type VerifiedContextReader interface {
	massifs.VerifiedContextReader
}

type VerifiedReplica struct {
	cCtx         *cli.Context
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
	if massifHeight > massifHeightMax {
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
		cCtx:         cCtx,
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
	tenantIdentity string, startMassif, endMassif uint32) error {

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

	// on demand promotion of a v0 state to a v1 state, for compatibility with the consistency check.
	trustedBaseState := func(local *massifs.VerifiedContext) (massifs.MMRState, error) {

		if local.MMRState.Version > int(massifs.MMRStateVersion0) {
			return local.MMRState, nil
		}

		// At this point we have a local seal in v0 format and we expect the
		// remote seal to be in v1 format.
		// We need to promote the legacy base state to a V1 state for the
		// consistency check.  This is a one way operation, and the legacy seal
		// root is discarded.  Once the seal for the open massif is upgraded,
		// this case will never be encountered again for that tenant.

		peaks, err := mmr.PeakHashes(local, local.MMRState.MMRSize-1)
		if err != nil {
			return massifs.MMRState{}, err
		}
		root := mmr.HashPeaksRHS(sha256.New(), peaks)
		if !bytes.Equal(root, local.MMRState.LegacySealRoot) {
			return massifs.MMRState{}, fmt.Errorf("legacy seal root does not match the bagged peaks")
		}
		state := local.MMRState
		state.Version = int(massifs.MMRStateVersion1)
		// Keep the legacy seal root so that we can verify in the case where the remote is a V0 seal
		// state.LegacySealRoot = nil
		state.Peaks = peaks
		return state, nil
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

	// We always verify up to the requested massif, but we do not re-verify
	// massifs we have already verified and replicated localy. If the last
	// locally replicated masif is ahead of the endMassif we do nothing here.
	//
	// The --ancestors option is used to ensure there is a minimum number of
	// verified massifs replicated locally, and influnces the startMassif to
	// acheive this.
	//
	// The startMassif is the greater of the requested start and the massif
	// index of the last locally verified massif.  Our verification always reads
	// the remote massifs starting from the startMassif.
	//
	// In the loop below we ensure three key things:
	// 1. If there is a local replica of the remote, we ensure the remote is
	//   consistent with the replica.
	// 2. If the remote starts a new massif, and we locally have its
	//    predecessor, we ensure the remote is consistent with the local predecessor.
	// 3. If there is no local replica, we create one by copying the the remote.
	//
	// Note that we arrange things so that local is always the last avaible
	// local massif, or nil.  When dealing with the remote corresponding to
	// startMassif, the local is *either* the predecessor or is the incomplete
	// local replica of the remote being considered. After the first remote is
	// dealt with, local is always the predecessor.

	if local != nil {

		// Start from the next massif after the last verified massif and do not
		// re-verify massifs we have already verified and replicated,
		if startMassif > local.Start.MassifIndex+1 {
			// if the start of the ancestors is more than one massif ahead of
			// the local, then we start afresh.
			local = nil
		} else {
			startMassif = local.Start.MassifIndex
		}
	}

	for i := startMassif; i <= endMassif; i++ {

		remoteVerifyOpts := []massifs.ReaderOption{massifs.WithCBORCodec(v.cborCodec)}
		if local != nil {
			// Promote the trusted base state to a V1 state if it is a V0 state.
			baseState, err := trustedBaseState(local)
			if err != nil {
				return err
			}
			remoteVerifyOpts = append(remoteVerifyOpts, massifs.WithTrustedBaseState(baseState))
		}

		// On the first iteration local is *either* the predecessor to
		// startMassif or it is the, as yet, incomplete local replica of it.
		// After the first iteration, local is always the predecessor. (If the
		// remote is still incomplte it means there is no subseqent massif to
		// read)
		remote, err := v.remoteReader.GetVerifiedContext(
			ctx, tenantIdentity, uint64(i), remoteVerifyOpts...)
		if err != nil {
			// both the remote massif and it's seal must be present for the
			// verification to succeed, so we don't filter using isBlobNotFound
			// here.
			return err
		}

		// next round, use the just replicated remote as the trusted base for verification

		// read the local massif, if it exists, reading at the end of the loop
		local, err = v.localReader.GetVerifiedContext(ctx, tenantIdentity, uint64(i))
		if !isNilOrNotFound(err) {
			return err
		}

		// copy the remote locally, safely replacing the coresponding local if one exists
		err = v.replicateVerifiedContext(local, remote)
		if err != nil {
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
func (v *VerifiedReplica) replicateVerifiedContext(
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
		// note: the length equal check is elevated so we only write to local
		// disc if there are changes.  this duplicates a check in
		// verifiedStateEqual in the interest of avoiding accidents due to
		// future refactorings.
		if !verifiedStateEqual(local, remote) {
			return fmt.Errorf("%w: %s, massif=%d", ErrRemoteLogInconsistentRootState, tenantIdentity, massifIndex)
		}
		return nil
	}

	return v.localReader.ReplaceVerifiedContext(remote, v.writeOpener)
}

func verifiedStateEqual(a *massifs.VerifiedContext, b *massifs.VerifiedContext) bool {

	var err error

	// There is no difference in the log format between the two versions currently supported.
	if len(a.Data) != len(b.Data) {
		return false
	}
	fromRoots := a.ConsistentRoots
	toRoots := b.ConsistentRoots
	// If either state is a V0 state, compare the legacy seal roots
	if a.MMRState.Version == int(massifs.MMRStateVersion0) || b.MMRState.Version == int(massifs.MMRStateVersion0) {
		rootA := peakBaggedRoot(a.MMRState)
		rootB := peakBaggedRoot(b.MMRState)
		if !bytes.Equal(rootA, rootB) {
			return false
		}
		if a.MMRState.Version == int(massifs.MMRStateVersion0) {
			fromRoots, err = mmr.PeakHashes(a, a.MMRState.MMRSize-1)
			if err != nil {
				return false
			}
		}
		if b.MMRState.Version == int(massifs.MMRStateVersion0) {
			toRoots, err = mmr.PeakHashes(b, b.MMRState.MMRSize-1)
			if err != nil {
				return false
			}
		}

	}

	// If both states are V1 states, compare the peaks
	if len(fromRoots) != len(toRoots) {
		return false
	}
	for i := range len(fromRoots) {
		if !bytes.Equal(fromRoots[i], toRoots[i]) {
			return false
		}
	}
	return true
}

// peakBaggedRoot is used to obtain an MMRState V0 bagged root from a V1 accumulator peak list.
// If a v0 state is provided, the root is returned as is.
func peakBaggedRoot(state massifs.MMRState) []byte {
	if state.Version < int(massifs.MMRStateVersion1) {
		return state.LegacySealRoot
	}
	return mmr.HashPeaksRHS(sha256.New(), state.Peaks)
}

type changeCollector struct {
	log         logger.Logger
	watchOutput string
}

func (c *changeCollector) Logf(msg string, args ...any) {
	if c.log == nil {
		return
	}
	c.log.Infof(msg, args...)
}

func (c *changeCollector) Outf(msg string, args ...any) {
	c.watchOutput += fmt.Sprintf(msg, args...)
}

func newWatchConfig(cCtx *cli.Context, cmd *CmdCtx) (WatchConfig, error) {
	cfg := WatchConfig{
		WatchCount: 1,
		WatchConfig: watcher.WatchConfig{
			Horizon: tenYearsOfHours,
		},
	}
	err := watcher.ConfigDefaults(&cfg.WatchConfig)
	if err != nil {
		return WatchConfig{}, err
	}
	cfg.ReaderURL = cmd.readerURL

	tenants := CtxGetTenantOptions(cCtx)
	if len(tenants) == 0 {
		return cfg, nil
	}
	cfg.WatchTenants = make(map[string]bool)
	for _, t := range tenants {
		cfg.WatchTenants[strings.TrimPrefix(t, tenantPrefix)] = true
	}
	return cfg, nil
}

func readTenantMassifChanges(ctx context.Context, cCtx *cli.Context, cmd *CmdCtx) ([]TenantMassif, error) {

	if cCtx.IsSet("latest") {
		// This is because people get tripped up with the `veracity watch -z 90000h | veracity replicate-logs` idiom,
		// Its such a common use case that we should just make it work.
		cfg, err := newWatchConfig(cCtx, cmd)
		if err != nil {
			return nil, err
		}
		forceProdUrl := cCtx.String("data-url") == ""

		reader, err := cfgReader(cmd, cCtx, forceProdUrl)
		if err != nil {
			return nil, err
		}

		collector := &changeCollector{log: cmd.log}
		err = WatchForChanges(ctx, cfg, reader, collector)
		if err != nil {
			return nil, err
		}

		return scannerToTenantMassifs(bufio.NewScanner(strings.NewReader(collector.watchOutput)))
	}

	tenants := CtxGetTenantOptions(cCtx)
	if len(tenants) == 1 {
		return []TenantMassif{{Tenant: tenants[0], Massif: cCtx.Int("massif")}}, nil
	}
	if len(tenants) > 1 {
		return nil, fmt.Errorf("multiple tenants may only be used with --latest")
	}

	// If --changes is set the tenants and massif indices are read from the identified file
	changesFile := cCtx.String("changes")
	if changesFile != "" {
		return filePathToTenantMassifs(changesFile)
	}

	// No explicit config and --all not set, read from stdin
	return stdinToDecodedTenantMassifs()
}
