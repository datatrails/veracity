package veracity

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/veraison/go-cose"

	commoncose "github.com/datatrails/go-datatrails-common/cose"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/veracity/keyio"
	"github.com/datatrails/veracity/localmassifs"
	"github.com/datatrails/veracity/scitt"
	"github.com/urfave/cli/v2"
)

// coseSigner implements IdentifiableCoseSigner
type identifiableCoseSigner struct {
	innerSigner cose.Signer
	publicKey   ecdsa.PublicKey
}

func (s *identifiableCoseSigner) Algorithm() cose.Algorithm {
	return s.innerSigner.Algorithm()
}

func (s *identifiableCoseSigner) Sign(rand io.Reader, content []byte) ([]byte, error) {
	return s.innerSigner.Sign(rand, content)
}

func (s *identifiableCoseSigner) LatestPublicKey() (*ecdsa.PublicKey, error) {
	return &s.publicKey, nil
}

func (s *identifiableCoseSigner) PublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	return &s.publicKey, nil
}

func (s *identifiableCoseSigner) KeyLocation() string {
	return "robinbryce.me"
}

func (s *identifiableCoseSigner) KeyIdentifier() string {
	// the returned kid needs to match the kid format of the keyvault key
	return "location:robinbryce/version1"
}

// NewAppendCmd appends an entry to a local ledger, optionally sealing it with a provided private key.
func NewAppendCmd() *cli.Command {
	return &cli.Command{
		Name:  "append",
		Usage: "add an entry to a local ledger, optionally sealing it with a provided private key",
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name: "mmrindex", Aliases: []string{"i"},
			},
			&cli.Int64Flag{
				Name: "massif", Aliases: []string{"m"},
				Usage: "allow inspection of an arbitrary mmr index by explicitly specifying a massif index",
				Value: -1,
			},
			&cli.StringFlag{
				Name:  "sealer-key",
				Usage: "the sealer key to use for signing the entry, in cose .cbor. Only P-256, ES256 is supported. If --generate-sealer-key is set, this generated key will be written to this file.",
			},
			&cli.StringFlag{
				Name:  "sealer-key-pem",
				Usage: "the sealer key to use for signing the entry, in PEM format. Only P-256, ES256 is supported. If --generate-sealer-key is set, this generated key will be written to this file.",
			},
			&cli.StringFlag{
				Name:  "sealer-public-key-pem",
				Usage: "If set, and if the sealer key is generated, the public key in PEM format is saved to this file.",
			},

			&cli.StringFlag{
				Name:  "trusted-sealer-key-pem",
				Usage: "verify the current seal using this pem file based public key",
			},

			&cli.StringFlag{
				Name:  "receipt-file",
				Usage: "file name to write the receipt to, defaults to 'receipt-{mmrIndex}.cbor'",
			},

			&cli.StringFlag{
				Name:  "signed-statement",
				Usage: "read statement to register from this file. if statements-dir is also set, this statement is registered first, then all statements in the directory are registered.",
			},
			&cli.StringFlag{
				Name:  "statements-dir",
				Usage: "read statements to register from this directory. the statements are added in lexical filename order",
			},

			&cli.BoolFlag{
				Name:  "generate-sealer-key",
				Usage: "generate an ephemeral sealer key and write it to the sealer-key file. If the sealer-key file already exists, it will be overwritten. the default file name is 'ecdsa-key-private.cbor'.",
			},
			&cli.StringFlag{
				Name:  "massifs-dir",
				Usage: "the directory to read the massifs from.",
			},
			&cli.StringFlag{
				Name:  "seals-dir",
				Usage: "the directory to read the massif seals from.",
			},
		},
		Action: func(cCtx *cli.Context) error {
			var err error
			cmd := &CmdCtx{}

			if !cCtx.IsSet("data-local") {
				return errors.New("this command supports local replicas only, and requires --data-local")
			}
			err = cfgLogging(cmd, cCtx)
			if err != nil {
				return fmt.Errorf("failed to configure logging: %w", err)
			}
			err = cfgIDState(cmd, cCtx)
			if err != nil {
				return fmt.Errorf("failed to configure id state: %w", err)
			}
			if cmd.cborCodec, err = massifs.NewRootSignerCodec(); err != nil {
				return err
			}

			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}

			//
			// Read or generate a key to seal the forked log
			//
			var sealingKey *ecdsa.PrivateKey
			if cCtx.IsSet("sealer-key") && !cCtx.Bool("generate-sealer-key") {
				sealerKeyFile := cCtx.String("sealer-key")
				if sealerKeyFile == "" {
					return errors.New("sealer-key file is required")
				}
				sealingKey, err = keyio.ReadECDSAPrivateCose(sealerKeyFile, "P-256")
				if err != nil {
					return fmt.Errorf("failed to load sealer key from file %s: %w", sealerKeyFile, err)
				}
			}
			if cCtx.IsSet("sealer-key-pem") && !cCtx.Bool("generate-sealer-key") {
				if cCtx.IsSet("sealer-key") {
					fmt.Printf("verifying with sealer-key-pem %s (in preference to sealer-key)", cCtx.String("sealer-key-pem"))
				}
				sealerKeyFile := cCtx.String("sealer-key-pem")
				if sealerKeyFile == "" {
					return errors.New("sealer-key file is required")
				}
				sealingKey, err = keyio.ReadECDSAPrivatePEM(sealerKeyFile)
				if err != nil {
					return fmt.Errorf("failed to load sealer key from file %s: %w", sealerKeyFile, err)
				}
			}

			if cCtx.Bool("generate-sealer-key") {
				sealingKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					return err
				}
			}

			//
			// Read the head of a locally replicated production datatrails ledger
			//
			readerCfg, err := localmassifs.NewReaderDefaultConfig(
				cmd.log,
				cCtx.String("massifs-dir"),
				cCtx.String("seals-dir"),
			)
			if err != nil {
				return fmt.Errorf("failed to create massif reader config: %w", err)
			}
			verified, err := localmassifs.ReadVerifiedHeadMassif(readerCfg)
			if err != nil {
				return fmt.Errorf("failed to read verified head massif: %w", err)
			}

			mmrSizeOrig := verified.RangeCount()
			fmt.Printf("%8d verified-size\n", mmrSizeOrig)
			verified.Tags = map[string]string{}

			//
			// Add a batch of statements, including the in-toto from ammoury
			//
			statements, err := addStatements(cmd, cCtx, &verified.MassifContext)
			if err != nil {
				return err
			}
			fmt.Printf("%d statements registered\n", len(statements))
			mmrSizeNew := verified.RangeCount()
			peakHashesNew, err := mmr.PeakHashes(&verified.MassifContext, mmrSizeNew-1)
			if err != nil {
				return err
			}
			for i, peak := range peakHashesNew {
				fmt.Printf("peak[%d]: %x\n", i, peak)
			}

			alg, err := commoncose.CoseAlgForEC(sealingKey.PublicKey)
			if err != nil {
				return err
			}

			coseSigner, err := cose.NewSigner(alg, sealingKey)
			if err != nil {
				return err
			}
			identifiableSigner := &identifiableCoseSigner{
				innerSigner: coseSigner,
				publicKey:   sealingKey.PublicKey,
			}

			//
			// Seal  a checkpoint for the locally forked ledger with a made up sealing key
			// Receipts are rooted at a checkpoint accumulator state.
			//
			rootSigner := massifs.NewRootSigner("https://github.com/robinbryce/veracity", cmd.cborCodec)

			// TODO: account for filling a massif
			mmrSizeCurrent := verified.RangeCount()
			cp, err := mmr.IndexConsistencyProof(&verified.MassifContext, verified.MMRState.MMRSize-1, mmrSizeCurrent-1)
			if err != nil {
				return err
			}

			// To create the checkpoint, we first check that the current state
			// contains the previously verified state. This necessarily produces
			// and verifies the new accumulator which we can then include with
			// the new checkpoint.
			ok, peaksB, err := mmr.CheckConsistency(
				verified, sha256.New(),
				cp.MMRSizeA, cp.MMRSizeB, verified.MMRState.Peaks)
			if !ok {
				return fmt.Errorf("consistency check failed: verify failed")
			}
			if err != nil {
				return err
			}
			lastIDTimestamp := verified.GetLastIdTimestamp()

			state := massifs.MMRState{
				Version:         int(massifs.MMRStateVersionCurrent),
				MMRSize:         mmrSizeCurrent,
				Peaks:           peaksB,
				Timestamp:       time.Now().UnixMilli(),
				CommitmentEpoch: verified.MMRState.CommitmentEpoch,
				IDTimestamp:     lastIDTimestamp,
			}

			//
			// Read and decode the checkpoint
			//
			// Given a signed checkpoint, receipts can be self served for any
			// element included in the MMR before that checkpoint.  Leaves from
			// the massif corresponding to the massif need no other data.
			// Leaves from earlier massifs *may* need the earlier massif, but
			// often don't  (its deterministic and computable when and which
			// earlier massifs are needed for an arbitrary mmrIndex)
			//
			// It is never necessary to have more than two massifs in order to
			// produce a receipt against the latest checkpoint.
			//
			// There is no particular reason to re-fresh, or even save, receipts
			// if you have a trustworthy store of checkpoints.
			//
			mmrStatement := statements[0]
			// A more appropriate subject would be the identity of the log ...
			subject := fmt.Sprintf("fork-%d-%d.bin", verified.MMRState.MMRSize-1, mmrSizeCurrent)
			publicKey, err := identifiableSigner.LatestPublicKey()
			if err != nil {
				return fmt.Errorf("unable to get public key for signing key %w", err)
			}

			keyIdentifier := identifiableSigner.KeyIdentifier()
			data, err := rootSigner.Sign1(coseSigner, keyIdentifier, publicKey, subject, state, nil)
			if err != nil {
				return err
			}

			// note that state is not verified here, but we just signed it so it is our droid
			msg, state, err := massifs.DecodeSignedRoot(cmd.cborCodec, data)
			if err != nil {
				return err
			}

			//
			// Generate the inclusion proof, note that we don't actually need
			// the leaf hash to do this.  So *anyone* can obtain a receipt for
			// *any* leaf at any time, given only the specific massif (tile)
			// that leaf was registered in.  and its associated checkpoint.
			//
			proof, err := mmr.InclusionProof(&verified.MassifContext, state.MMRSize-1, mmrStatement.MMRIndexLeaf)
			if err != nil {
				return fmt.Errorf(
					"failed to generating inclusion proof: %d in MMR(%d), %v",
					mmrStatement.MMRIndexLeaf, verified.MMRState.MMRSize, err)
			}

			//
			// Locate the pre-signed receipt for the accumulator peak containing the leaf.
			//
			peakIndex := mmr.PeakIndex(mmr.LeafCount(state.MMRSize), len(proof))
			// NOTE: The old-accumulator compatibility property, from
			// https://eprint.iacr.org/2015/718.pdf, along with the COSE protected &
			// unprotected buckets, is why we can just pre sign the receipts.
			// As long as the receipt consumer is convinced of the logs consistency (not split view),
			// it does not matter which accumulator state the receipt is signed against.

			var peaksHeader massifs.MMRStateReceipts
			err = cbor.Unmarshal(msg.Headers.RawUnprotected, &peaksHeader)
			if err != nil {
				return fmt.Errorf(
					"%w: failed decoding peaks header", err)
			}
			if peakIndex >= len(peaksHeader.PeakReceipts) {
				return fmt.Errorf(
					"%w: peaks header contains to few peak receipts", err)
			}

			// This is an array of marshaled COSE_Sign1's
			receiptMsg := peaksHeader.PeakReceipts[peakIndex]
			signed, err := commoncose.NewCoseSign1MessageFromCBOR(
				receiptMsg, commoncose.WithDecOptions(massifs.CheckpointDecOptions()))
			if err != nil {
				return fmt.Errorf(
					"%w: failed to decode pre-signed receipt for MMR(%d)",
					err, state.MMRSize)
			}

			// To avoid creating invalid receipts due to bugs in this demo code, check the root matches the appropriate peak.
			root := mmr.IncludedRoot(sha256.New(), mmrStatement.MMRIndexLeaf, mmrStatement.LeafHash, proof)

			if !bytes.Equal(root, peaksB[peakIndex]) {
				return fmt.Errorf(
					"%w: root %x of leaf %d in MMR(%d) does not match peak %d %x",
					ErrVerifyInclusionFailed, root, mmrStatement.MMRIndexLeaf, state.MMRSize, peakIndex, state.Peaks[peakIndex])
			}

			//
			// Make the MMR draft receipt by attaching the inclusion proof to the Unprotected header
			//
			signed.Headers.RawUnprotected = nil

			verifiableProofs := massifs.MMRiverVerifiableProofs{
				InclusionProofs: []massifs.MMRiverInclusionProof{{
					Index:         mmrStatement.MMRIndexLeaf,
					InclusionPath: proof,
				}},
			}

			tagOriginSubject := int64(-257)
			tagOriginIssuer := tagOriginSubject - 1
			tagLeafHash := tagOriginSubject - 2
			tagIDTimestamp := tagOriginSubject - 3
			tagExtraBytes := tagOriginSubject - 4

			signed.Headers.Unprotected[massifs.VDSCoseReceiptProofsTag] = verifiableProofs
			// these values would usually be provided by the application, or obtained directly from any replica.
			// the unprotected headers are not signed, and are intended for this sort of convenience.
			signed.Headers.Unprotected[tagOriginIssuer] = mmrStatement.Claims.Issuer
			signed.Headers.Unprotected[tagOriginSubject] = mmrStatement.Claims.Subject
			signed.Headers.Unprotected[tagIDTimestamp] = mmrStatement.IDTimestamp
			signed.Headers.Unprotected[tagExtraBytes] = mmrStatement.ExtraBytes
			signed.Headers.Unprotected[tagLeafHash] = mmrStatement.LeafHash
			//
			// Save the receipt to a file
			//
			receiptCbor, err := signed.MarshalCBOR()
			if err != nil {
				return fmt.Errorf("failed to marshal receipt: %w", err)
			}

			receiptFileName := cCtx.String("receipt-file")
			if receiptFileName == "" {
				receiptFileName = fmt.Sprintf("receipt-%d.cbor", mmrStatement.MMRIndexLeaf)
			}
			if err := os.WriteFile(receiptFileName, receiptCbor, os.FileMode(0644)); err != nil {
				return fmt.Errorf("failed to write receipt file %s: %w", receiptFileName, err)
			}
			fmt.Printf("wrote receipt file %s\n", receiptFileName)

			//
			// A bunch of persistence conveniences for the sake of the demo
			//

			forkFileName := filepath.Join(".", fmt.Sprintf("fork-%d-%d.bin", verified.MMRState.MMRSize-1, mmrSizeCurrent))
			if err := os.WriteFile(forkFileName, data, os.FileMode(0644)); err != nil {
				return fmt.Errorf("failed to write log fork file %s: %w", forkFileName, err)
			}
			fmt.Printf("wrote forked log massif file %s\n", forkFileName)

			checkpointFileName := filepath.Join(".", fmt.Sprintf("checkpoint-%d.cbor", mmrSizeCurrent))
			if err := os.WriteFile(checkpointFileName, data, os.FileMode(0644)); err != nil {
				return fmt.Errorf("failed to write checkpoint file %s: %w", checkpointFileName, err)
			}
			fmt.Printf("wrote checkpoint file %s\n", checkpointFileName)
			if cCtx.Bool("generate-sealer-key") {
				// write the sealer key to the sealer-key file
				sealerKeyFile := cCtx.String("sealer-key")
				if sealerKeyFile == "" {
					sealerKeyFile = keyio.ECDSAPrivateDefaultFileName
				}
				if _, err := keyio.WriteECDSAPrivateCOSE(sealerKeyFile, sealingKey); err != nil {
					return fmt.Errorf("failed to write sealer key to file %s: %w", sealerKeyFile, err)
				}
				fmt.Printf("wrote sealer key to file %s\n", sealerKeyFile)
				sealerKeyFile = cCtx.String("sealer-key-pem")
				if sealerKeyFile == "" {
					sealerKeyFile = keyio.ECDSAPrivateDefaultPEMFileName
				}
				if err := keyio.WriteECDSAPrivatePEM(sealerKeyFile, sealingKey); err != nil {
					return fmt.Errorf("failed to write sealer key to file %s: %w", sealerKeyFile, err)
				}
				fmt.Printf("wrote sealer key to file %s\n", sealerKeyFile)

				sealerKeyFile = cCtx.String("sealer-public-key-pem")
				if sealerKeyFile == "" {
					sealerKeyFile = keyio.ECDSAPublicDefaultPEMFileName
				}
				if _, err := keyio.WriteCoseECDSAPublicKey(sealerKeyFile, &sealingKey.PublicKey); err != nil {
					return fmt.Errorf("failed to write sealer key to file %s: %w", sealerKeyFile, err)
				}
				fmt.Printf("wrote sealer public key to file %s\n", sealerKeyFile)
			}
			return nil
		},
	}
}

// addStatements adds the signed statements to the massif and returns the leaf
// indices of the added statements.
// If a specific statement is specified via --signed-statement, then it is
// added first. THose discovered from --statement-dir are added in lexical
// filename order.
func addStatements(cmd *CmdCtx, cCtx *cli.Context, massif *massifs.MassifContext) ([]scitt.MMRStatement, error) {
	var fileNames []string
	var statements []scitt.MMRStatement

	if cCtx.String("signed-statement") != "" {
		fileNames = append(fileNames, cCtx.String("signed-statement"))
	}

	if cCtx.String("statements-dir") != "" {
		files, err := listFilesWithSuffix(cCtx.String("statements-dir"), ".cbor")
		if err != nil {
			return nil, err
		}
		fileNames = append(fileNames, files...)
	}
	if len(fileNames) == 0 {
		return nil, fmt.Errorf("no signed statements found, please specify --signed-statement or --statements-dir or both")
	}

	for _, fileName := range fileNames {
		mmrStatement, err := readStatementFromFile(fileName, cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to read signed statement from file %s: %w", cCtx.String("signed-statement"), err)
		}

		// the *next* index to be added is the current *count*
		mmrStatement.MMRIndexLeaf = massif.RangeCount()

		_, err = massif.AddHashedLeaf(
			sha256.New(),
			mmrStatement.IDTimestamp,
			mmrStatement.ExtraBytes,
			// use the issuer as the origin log id, which isn't quite right, but is close enough for this demo
			[]byte(mmrStatement.Claims.Issuer),
			[]byte("scitt"),
			mmrStatement.LeafHash,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to add hashed leaf: %w", err)
		}

		statements = append(statements, *mmrStatement)

		fmt.Printf("index             : %d\n", mmrStatement.MMRIndexLeaf)
		fmt.Printf(" issuer           : %s\n", mmrStatement.Claims.Issuer)
		fmt.Printf(" idtimestamp      : %x\n", mmrStatement.IDTimestamp)
		fmt.Printf(" extraBytes       : %x\n", mmrStatement.ExtraBytes)
		fmt.Printf(" leaf-hash        : %x\n", mmrStatement.LeafHash)
		fmt.Printf(" statement-hash   : %x\n", mmrStatement.Hash)
		fmt.Printf(" node count       : %d\n", (len(massif.Data)-int(massif.LogStart()))/32)

		value, err := massif.Get(mmrStatement.MMRIndexLeaf)
		if err != nil {
			return nil, fmt.Errorf("failed to get leaf value for index %d: %w", mmrStatement.MMRIndexLeaf, err)
		}
		if !bytes.Equal(value, mmrStatement.LeafHash) {
			// this will mean a bug in the hacked up code if it catches
			return nil, fmt.Errorf("leaf hash %x does not match expected value %x for index %d",
				value, mmrStatement.LeafHash, mmrStatement.MMRIndexLeaf)
		}
	}
	return statements, nil
}

func readStatementFromFile(fileName string, cmd *CmdCtx) (*scitt.MMRStatement, error) {
	mmrStatement, cpd, err := scitt.NewMMRStatementFromFile(fileName, cmd, scitt.RegistrationPolicyVerified())
	if err != nil {
		if cpd != nil && cpd.Instance != scitt.ProblemInstanceConfirmationMissing {
			return nil, fmt.Errorf("%w: failed reading and checking signed statement: %s", err, cpd.Detail)
		}
		// for demo purposes, because we do not support x509
		mmrStatement, cpd, err = scitt.NewMMRStatementFromFile(fileName, cmd, scitt.RegistrationPolicyUnverified())
		if err != nil {
			return nil, fmt.Errorf("%w: failed reading and checking signed statement: %s", err, cpd.Detail)
		}
		err = nil
		cpd = nil
	}
	return mmrStatement, nil
}

func listFilesWithSuffix(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), suffix) {
			matches = append(matches, filepath.Join(dir, entry.Name()))
		}
	}
	return matches, nil
}
