package veracity

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-common/cose"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/urfave/cli/v2"
)

var (
	ErrNewReplicaNotEmpty             = errors.New("the local directory for a new replica already exists")
	ErrWriteIncomplete                = errors.New("a file write succeeded, but the number of bytes written was shorter than the supplied data")
	ErrSealNotFound                   = errors.New("seal not found")
	ErrSealVerifyFailed               = errors.New("the seal signature verification failed")
	ErrFailedCheckingConsistencyProof = errors.New("failed to check a consistency proof")
	ErrFailedToCreateReplicaDir       = errors.New("failed to create a directory needed for local replication")
	ErrRequiredOption                 = errors.New("a required option was not provided")
	ErrRemoteSealKeyMatchFailed       = errors.New("the provided public key did not match the remote sealing key")
	ErrRemoteLogTruncated             = errors.New("the local replica indicates the remote log has been truncated")
	ErrRemoteLogInconsistentRootState = errors.New("the local replica root state disagrees with the remote")
)

var (
	DefaultCacheDirMode = os.FileMode(0755)
)

// NewVerifyConsistencyCmd verifies consistency of an updated log against trusted copy of an earlier state.
//
//nolint:gocognit
func NewVerifyConsistencyCmd() *cli.Command {
	return &cli.Command{
		Name:    "verify-consistency",
		Aliases: []string{"consistent"},
		Usage:   `verify the consistency of a remote log update against a trusted copy of an earlier merkle log state.`,
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
				Name: "public-sealer-key",
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

			opts := []VerifierOption{
				WithReplicaDir(cCtx.String("replicadir")),
				WithBlobReader(reader),
			}
			pemString := cCtx.String("public-sealer-key")
			if pemString != "" {
				pem, err := DecodeECDSAPublicString(pemString)
				if err != nil {
					return err
				}
				opts = append(opts, WithTrustedSealerPub(pem))
			}

			v, err := NewConsistencyVerifier(
				logger.Sugar,
				opts...,
			)
			if err != nil {
				return err
			}
			return v.VerifyConsistency(context.Background(), tenantIdentity, uint32(headMassifIndex))
		},
	}
}

type WriteOpener interface {
	Open(string) (io.WriteCloser, error)
}

// sealGetter supports reading a massif seal identified by a specific massif index
type sealGetter interface {
	GetSignedRoot(
		ctx context.Context, tenantIdentity string, massifIndex uint32,
		opts ...massifs.ReaderOption,
	) (*cose.CoseSign1Message, massifs.MMRState, error)
}

type ConsistencyVerifier struct {
	opts         VerifierOptions
	log          logger.Logger
	writeOpener  WriteOpener
	cborCodec    cbor.CBORCodec
	localReader  *massifs.LocalMassifReader
	remoteReader MassifReader
	sealGetter   sealGetter
}

func NewConsistencyVerifier(log logger.Logger, opts ...VerifierOption) (*ConsistencyVerifier, error) {
	cborCodec, err := massifs.NewRootSignerCodec()
	if err != nil {
		return nil, err
	}

	options := NewVerifierOptions()
	for _, o := range opts {
		o(&options)
	}
	if options.blobReader == nil {
		return nil, fmt.Errorf("%w: %s", ErrRequiredOption, "WithBlobReader must be provided")
	}

	localReader, err := massifs.NewLocalReader(
		logger.Sugar, NewFileOpener(),
		massifs.WithLocalReplicaDir(options.replicaDir),
		massifs.WithLocalMassifLister(NewDirLister()),
		massifs.WithLocalSealLister(NewDirLister()),
		massifs.WithReaderOption(massifs.WithCBORCodec(cborCodec)),
	)
	if err != nil {
		return nil, err
	}
	remoteReader := massifs.NewMassifReader(log, options.blobReader)
	sealReader := massifs.NewSignedRootReader(log, options.blobReader, cborCodec)

	return &ConsistencyVerifier{
		log:          log,
		opts:         options,
		writeOpener:  NewFileWriteOpener(),
		cborCodec:    cborCodec,
		localReader:  &localReader,
		remoteReader: &remoteReader,
		sealGetter:   &sealReader,
	}, nil
}

type VerifierOptions struct {
	replicaDir          string
	trustedSealerPubKey *ecdsa.PublicKey
	cacheDirMode        os.FileMode // 755 is the sensible default
	blobReader          azblob.Reader
}

type VerifierOption func(*VerifierOptions)

func NewVerifierOptions() VerifierOptions {
	return VerifierOptions{
		replicaDir:   ".",
		cacheDirMode: DefaultCacheDirMode,
	}
}

func WithReplicaDir(replicaDir string) VerifierOption {
	return func(opts *VerifierOptions) {
		opts.replicaDir = replicaDir
	}
}

func WithTrustedSealerPub(pub *ecdsa.PublicKey) VerifierOption {
	return func(opts *VerifierOptions) {
		opts.trustedSealerPubKey = pub
	}
}

func WithBlobReader(reader azblob.Reader) VerifierOption {
	return func(opts *VerifierOptions) {
		opts.blobReader = reader
	}
}

type logState struct {
	massif massifs.MassifContext
	seal   massifs.SealedState
}

// VerifyConsistency confirms that any additions to the remote log are
// consistent with the local replica Only the most recent local massif and seal
// need be retained for verification purposes.  If independent, off line,
// verification of inclusion is desired, retain as much of the log as is
// interesting.
func (v *ConsistencyVerifier) VerifyConsistency(
	ctx context.Context,
	tenantIdentity string, headMassifIndex uint32) error {

	if err := v.ensureTenantReplicaDirs(tenantIdentity); err != nil {
		return err
	}

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
		if local == nil {
			return nil
		}
		return []massifs.ReaderOption{massifs.WithTrustedBaseState(local.MMRState)}
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
			ctx, tenantIdentity, uint64(i), remoteOptionsFromLocal(local)...)
		if err != nil {
			return err
		}

		err = v.replicateVerifiedContext(local, remote)
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
func (v *ConsistencyVerifier) replicateVerifiedContext(
	local *massifs.VerifiedContext, remote *massifs.VerifiedContext) error {

	if local == nil {
		return v.replicateRemoteVerifiedContext(remote)
	}

	if local.TenantIdentity != remote.TenantIdentity {
		return fmt.Errorf("can't replace, tenant identies don't match: local %s vs remote %s", local.TenantIdentity, remote.TenantIdentity)
	}

	if local.Start.MassifIndex != remote.Start.MassifIndex {
		return fmt.Errorf(
			"can't replace, massif indices don't match: local %d vs remote %s",
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

	// remote & local should have identical values, we don't need to re-check that here tho
	logFilename, err := v.GetMassifLocalPath(tenantIdentity, massifIndex)
	// XXX: TODO: only write the portion of mc.Data that is covered by the seal
	// OR: verify consistency of the remainder
	// Unconditionally write the first massif to the local directory cache location
	err = writeAll(v.writeOpener, logFilename, remote.Data)
	if err != nil {
		return err
	}

	// Populate the cache record for the file we just wrote, this does not re-read the file
	return v.localReader.ReplaceMassif(logFilename, remote)
}

// replicateRemoteVerifiedContext unconditionally replicates the verified remote context to the local store
func (v *ConsistencyVerifier) replicateRemoteVerifiedContext(remote *massifs.VerifiedContext) error {
	logFilename, err := v.GetMassifLocalPath(remote.TenantIdentity, remote.Start.MassifIndex)
	err = writeAll(v.writeOpener, logFilename, remote.Data)
	if err != nil {
		return err
	}

	sealFilename, err := v.GetSealLocalPath(remote.TenantIdentity, remote.Start.MassifIndex)
	if err != nil {
		return err
	}
	sealBytes, err := remote.Sign1Message.MarshalCBOR()
	if err != nil {
		return err
	}
	err = writeAll(v.writeOpener, sealFilename, sealBytes)
	if err != nil {
		return err
	}
	return v.localReader.AddSeal(sealFilename, remote.Start.MassifIndex, &massifs.SealedState{
		Sign1Message: remote.Sign1Message,
		MMRState:     remote.MMRState,
	})
}

func (v *ConsistencyVerifier) ensureTenantReplicaDirs(tenantIdentity string) error {
	if !v.localReader.InReplicaMode() {
		return fmt.Errorf("replica dir must be configured on the local reader")
	}
	massifsDir := filepath.Join(v.localReader.GetReplicaDir(), v.GetRelativeMassifPath(tenantIdentity, 0))
	sealsDir := filepath.Join(v.localReader.GetReplicaDir(), v.GetRelativeSealPath(tenantIdentity, 0))
	err := os.MkdirAll(massifsDir, v.opts.cacheDirMode)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrFailedToCreateReplicaDir, massifsDir)

	}
	err = os.MkdirAll(sealsDir, v.opts.cacheDirMode)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrFailedToCreateReplicaDir, sealsDir)

	}
	return nil
}

// like logconfirmer  confirmForMassif
func (v *ConsistencyVerifier) replicateMassif(
	ctx context.Context,
	tenantIdentity string,
	lastSignedState massifs.MMRState,
	massifIndex uint32,
) (
	logState, error,
) {
	var err error
	s := logState{}

	// Read the next remote massif
	s.massif, err = v.remoteReader.GetMassif(ctx, tenantIdentity, uint64(massifIndex))
	if err != nil {
		return logState{}, err
	}
	logFilename, err := v.GetMassifLocalPath(s.massif.TenantIdentity, s.massif.Start.MassifIndex)
	if err != nil {
		return logState{}, err
	}

	// If a directory entry already exists in the cache, it is an error.
	// replicating the first skips the consistency check on the basis that there
	// is nothing to verify against. For this case, we do not do any fussy remote
	// vs local comparisons, we just require that the directory is empty.
	if _, ok := v.localReader.GetDirEntry(logFilename); ok {
		return logState{}, fmt.Errorf("%w: %s %s", tenantIdentity, logFilename)
	}

	// check the next remote is consistent with the previous local state
	ok, err := v.checkConsistency(lastSignedState, s.massif)
	if err != nil {
		return logState{}, err
	}
	if !ok {
		return logState{},
			fmt.Errorf("%w: proof verification check failed: err=%s, tenant=%s, massif=%d",
				ErrFailedCheckingConsistencyProof,
				err.Error(), tenantIdentity, s.massif.Start.MassifIndex)
	}

	// we have just confirmed that the new massif is consistent with the
	// *locally* held sealed state for the previous massif.  now get the remote
	// seal for the new massif. as we know the massif data is consistent with
	// the previous seal we just need to reproduce the seal payload and verify
	// the signature. If we are subsequently called for nextMassifIndex + 1,
	// this seal becomes our next local and the subsequent massif is checked for
	// consistency against it and so on ...

	msg, state, err := v.GetVerifiedMassifSeal(ctx, s.massif)
	if err != nil {
		return logState{}, err
	}

	err = writeAll(v.writeOpener, logFilename, s.massif.Data)
	if err != nil {
		return logState{}, err
	}

	// Populate the cache record for the file we just wrote, this does not re-read the file
	err = v.localReader.ReplaceMassif(logFilename, &s.massif)
	if err != nil {
		return logState{}, err
	}

	// Now store the seal
	sealFilename, err := v.GetSealLocalPath(s.massif.TenantIdentity, s.massif.Start.MassifIndex)
	if err != nil {
		return logState{}, err
	}
	sealBytes, err := msg.MarshalCBOR()
	if err != nil {
		return logState{}, err
	}

	err = writeAll(v.writeOpener, sealFilename, sealBytes)
	if err != nil {
		return logState{}, err
	}

	s.seal.Sign1Message = *msg
	s.seal.MMRState = state

	err = v.localReader.AddSeal(sealFilename, s.massif.Start.MassifIndex, &s.seal)
	if err != nil {
		return logState{}, err
	}

	return s, nil
}

// like logconfirmer  confirmFirstMassif
func (v *ConsistencyVerifier) replicateFirstMassif(
	ctx context.Context,
	tenantIdentity string,
) (massifs.MassifContext, massifs.MMRState, error) {

	// First, check we can get the remote massif 0 and that a local copy does not already exist

	// We can't use GetFirstMassif here, instead we Use GetMassif with an
	// explicit index of 0. GetFirstMassif uses list to find the "first"
	// available blob at the path.  We don't have list publicly*. Until we
	// support merklelog re-configuration (eg changing the massif height), The
	// "first" will always be massifIndex 0.  After a log parameter change and
	// we have migrated the "head" blob to a new path in remote storage, we will
	// need to revise this a bit.
	// * we sort of do have list publicly,  due to how the permissions workout
	//   for the apim principal, but we don't want to encourage its use.
	mc, err := v.remoteReader.GetMassif(ctx, tenantIdentity, 0)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}
	logFilename, err := v.GetMassifLocalPath(mc.TenantIdentity, mc.Start.MassifIndex)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}

	// If a directory entry already exists in the cache, it is an error.
	// replicating the first skips the consistency check on the basis that there
	// is nothing to verify against. For this case, we do not do any fussy remote
	// vs local comparisons, we just require that the directory is empty.
	if _, ok := v.localReader.GetDirEntry(logFilename); ok {
		return massifs.MassifContext{}, massifs.MMRState{}, fmt.Errorf("%w: %s %s", tenantIdentity, logFilename)
	}

	msg, state, err := v.GetVerifiedMassifSeal(ctx, mc)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}
	// If we get here we have fully verified the seal. This required us to re produce the
	// signed payload from the log, and if the caller provided a trusted copy of
	// the seal public key, we have checked it matched the signer of the remote
	// seal.

	// Now, we check that any additions beyond the seal are also consistent with it.

	consistent, err := v.checkConsistency(state, mc)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}
	if !consistent {
		return massifs.MassifContext{}, massifs.MMRState{},
			fmt.Errorf("%w: proof verification check failed: err=%s, tenant=%s, massif=%d",
				ErrFailedCheckingConsistencyProof,
				err.Error(), mc.TenantIdentity, mc.Start.MassifIndex)
	}

	// XXX: TODO: only write the portion of mc.Data that is covered by the seal
	// OR: verify consistency of the remainder
	// Unconditionally write the first massif to the local directory cache location
	err = writeAll(v.writeOpener, logFilename, mc.Data)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}

	// Populate the cache record for the file we just wrote, this does not re-read the file
	err = v.localReader.ReplaceMassif(logFilename, &mc)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}

	// Now store the seal
	sealFilename, err := v.GetSealLocalPath(mc.TenantIdentity, mc.Start.MassifIndex)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}
	seal, err := msg.MarshalCBOR()
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}

	err = writeAll(v.writeOpener, sealFilename, seal)
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}

	err = v.localReader.AddSeal(sealFilename, mc.Start.MassifIndex, &massifs.SealedState{Sign1Message: *msg, MMRState: state})
	if err != nil {
		return massifs.MassifContext{}, massifs.MMRState{}, err
	}

	return mc, state, nil
}

func (v *ConsistencyVerifier) checkConsistency(lastSignedState massifs.MMRState, mc massifs.MassifContext) (bool, error) {

	mmrSizeCurrent := mc.RangeCount()

	// If the size has not advanced return the previously signed state.
	if mmrSizeCurrent == lastSignedState.MMRSize {
		// There are two cases of note, but we can treat them equivalently here
		// 1. The massif is complete
		// 2. There have been no further entries on an incomplete massif
		return true, nil
	}

	// If we reach this point, the log has grown and we must ensure the new log
	// data is an extension of the previously signed tree head.

	cp, err := mmr.IndexConsistencyProof(
		lastSignedState.MMRSize, mmrSizeCurrent, &mc, sha256.New())
	if err != nil {
		return false, fmt.Errorf(
			"%w: failed to produce proof. tenant=%s, massif=%d",
			ErrFailedCheckingConsistencyProof, mc.TenantIdentity, mc.Start.MassifIndex)
	}

	ok, _ /*rootB*/, err := mmr.CheckConsistency(&mc, sha256.New(), cp, lastSignedState.Root)
	if err != nil {
		return false,
			fmt.Errorf("%w: proof verification error: err=%s, tenant=%s, massif=%d",
				ErrFailedCheckingConsistencyProof,
				err.Error(), mc.TenantIdentity, mc.Start.MassifIndex)
	}

	return ok, nil
}

func (v *ConsistencyVerifier) GetSealLocalPath(tenantIdentity string, massifIndex uint32) (string, error) {

	relativePath := v.GetRelativeSealPath(tenantIdentity, massifIndex)

	// Note: ResolveDirectory does not error unless mustExist is set
	logDir, err := v.localReader.ResolveDirectory(relativePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(logDir, filepath.Base(relativePath)), nil
}

func (v *ConsistencyVerifier) GetRelativeSealPath(tenantIdentity string, massifIndex uint32) string {
	return strings.TrimPrefix(
		strings.TrimPrefix(
			massifs.V1MMRPrefix, v.GetSealBlobPath(tenantIdentity, massifIndex)), tenantIdentity)
}

func (v *ConsistencyVerifier) GetMassifLocalPath(
	tenantIdentity string, massifIndex uint32) (string, error) {

	relativePath := v.GetRelativeMassifPath(tenantIdentity, massifIndex)

	logDir, err := v.localReader.ResolveDirectory(relativePath)
	if err != nil {
		return "", err
	}

	return filepath.Join(logDir, filepath.Base(relativePath)), nil
}

func (v *ConsistencyVerifier) GetRelativeMassifPath(
	tenantIdentity string, massifIndex uint32) string {
	return strings.TrimPrefix(
		strings.TrimPrefix(
			massifs.V1MMRPrefix, v.GetMassifBlobPath(tenantIdentity, massifIndex)), tenantIdentity)
}

func (v *ConsistencyVerifier) GetSealBlobPath(tenantIdentity string, massifIndex uint32) string {
	return massifs.TenantMassifSignedRootPath(tenantIdentity, massifIndex)
}

func (v *ConsistencyVerifier) GetMassifBlobPath(tenantIdentity string, massifIndex uint32) string {
	return massifs.TenantMassifBlobPath(tenantIdentity, uint64(massifIndex))
}

func (v *ConsistencyVerifier) GetVerifiedMassifSeal(ctx context.Context, mc massifs.MassifContext) (*cose.CoseSign1Message, massifs.MMRState, error) {
	msg, state, err := v.GetMassifSeal(ctx, mc)
	if err != nil {
		return nil, massifs.MMRState{}, err
	}

	// NOTICE: The verification uses the public key that is provided on the
	// message.  If the caller wants to ensure the massif is signed by the
	// expected key then they must obtain a copy of the public key from a source
	// they trust and supply it as an option.

	pubKeyProvider := cose.NewCWTPublicKeyProvider(msg)

	if v.opts.trustedSealerPubKey != nil {
		remotePub, _, err := pubKeyProvider.PublicKey()
		if err != nil {
			return nil, massifs.MMRState{}, err
		}
		if !v.opts.trustedSealerPubKey.Equal(remotePub) {
			return nil, massifs.MMRState{}, ErrRemoteSealKeyMatchFailed
		}
	}

	err = massifs.VerifySignedRoot(
		v.cborCodec, pubKeyProvider,
		msg, state, nil,
	)
	return msg, state, err
}

func (v *ConsistencyVerifier) GetMassifSeal(ctx context.Context, mc massifs.MassifContext) (*cose.CoseSign1Message, massifs.MMRState, error) {
	msg, state, err := v.sealGetter.GetSignedRoot(ctx, mc.TenantIdentity, mc.Start.MassifIndex)
	if err != nil {
		return nil, massifs.MMRState{}, err
	}

	state.Root, err = mmr.GetRoot(state.MMRSize, &mc, sha256.New())
	if err != nil {
		return nil, massifs.MMRState{}, fmt.Errorf("%w: failed to get seal for massif 0 for tenant %s: %v", ErrSealNotFound, mc.TenantIdentity, err)
	}
	return msg, state, nil
}

func writeAll(wo WriteOpener, filename string, data []byte) error {
	f, err := wo.Open(filename)
	if err != nil {
		return err

	}
	defer f.Close()

	n, err := f.Write(data)
	if err != nil {
		return err
	}

	if n != len(data) {
		return fmt.Errorf("%w: %s", ErrWriteIncomplete, filename)
	}
	return nil
}
