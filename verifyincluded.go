package veracity

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-logverification/logverification/app"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/urfave/cli/v2"

	veracityapp "github.com/datatrails/veracity/app"
)

var (
	ErrVerifyInclusionFailed = errors.New("the entry is not in the log")
	ErrUncommittedEvents     = errors.New("one or more events did not have record of their inclusion in the log")
)

const (
	skipUncommittedFlagName = "skip-uncommitted"
)

func proofPath(proof [][]byte) string {
	var hexProof []string
	for _, node := range proof {
		hexProof = append(hexProof, fmt.Sprintf("%x", node))
	}
	return fmt.Sprintf("[%s]", strings.Join(hexProof, ", "))
}

// verifyEvent is an example function of how to verify the inclusion of a datatrails event using the mmr and massifs modules
func verifyEvent(
	event *app.AppEntry, logTenant string, mmrEntry []byte, massifHeight uint8, massifGetter MassifGetter,
) ([][]byte, error) {

	// Get the mmrIndex from the request and then compute the massif
	// it implies based on the massifHeight command line option.
	mmrIndex := event.MMRIndex()

	massifIndex := massifs.MassifIndexFromMMRIndex(massifHeight, mmrIndex)

	// read the massif blob
	massif, err := massifGetter.GetMassif(context.Background(), logTenant, massifIndex)
	if err != nil {
		return nil, err
	}

	mmrSize := massif.RangeCount()
	proof, err := mmr.InclusionProof(&massif, mmrSize, mmrIndex)
	if err != nil {
		return nil, err
	}

	// Note: we verify against the mmrSize of the massif which
	// includes the event. Future work can deepen this to include
	// discovery of the log head, and or verification against a
	// sealed MMRSize.
	verified, err := mmr.VerifyInclusion(&massif, sha256.New(), mmrSize, mmrEntry, mmrIndex, proof)
	if verified {
		return proof, nil
	}

	return nil, fmt.Errorf("%w: %v", ErrVerifyInclusionFailed, err)
}

// NewVerifyIncludedCmd verifies inclusion of a DataTrails event in the tenants Merkle Log
//
//nolint:gocognit
func NewVerifyIncludedCmd() *cli.Command {
	return &cli.Command{
		Name:    "verify-included",
		Aliases: []string{"included"},
		Usage: `verify the inclusion of an event, or list of events, in the tenant's merkle log.

The event response data from the DataTrails get event or list event queries can be provided directly.

See the README for example use.

Note: for publicly attested events, or shared protected events, you must use --tenant. Otherwise, the tenant is inferred from the event data.
`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: skipUncommittedFlagName, Value: false},
		},
		Action: func(cCtx *cli.Context) error {
			cmd := &CmdCtx{}

			var err error

			// This command uses the structured logger for all optional output.
			// Output not explicitly printed is silenced by default.
			if err = cfgLogging(cmd, cCtx); err != nil {
				return err
			}

			log := func(m string, args ...any) {
				cmd.log.Infof(m, args...)
			}

			tenantIdentity := cCtx.String("tenant")
			if tenantIdentity != "" {
				log("verifying for tenant: %s", tenantIdentity)
			} else {
				log("verifying protected events for the event creator")
			}

			// If we are reading the massif log locally, the log path is the
			// data-local path. The reader does the right thing regardless of
			// whether the option is a directory or a file.
			// verifyEvent defaults it to tenantIdentity for the benefit of the remote reader implementation
			tenantLogPath := cCtx.String("data-local")

			if tenantLogPath == "" {
				tenantLogPath = tenantIdentity
			}

			appData, err := veracityapp.ReadAppData(cCtx.Args().Len() == 0, cCtx.Args().Get(0))
			if err != nil {
				return err
			}

			verifiableLogEntries, err := veracityapp.AppDataToVerifiableLogEntries(appData, tenantIdentity)
			if err != nil {
				return err
			}

			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}

			var countNotCommitted int
			var countVerifyFailed int

			previousMassifIndex := uint64(0)
			var massifContext *massifs.MassifContext = nil

			for _, event := range verifiableLogEntries {

				leafIndex := mmr.LeafIndex(event.MMRIndex())

				// get the massif index for the event event
				massifIndex := massifs.MassifIndexFromMMRIndex(cmd.massifHeight, event.MMRIndex())

				// find the log tenant path if not provided
				if tenantLogPath == "" {

					var err error
					tenantLogPath, err = event.LogTenant()
					if err != nil {
						return err
					}

				}

				// check if we need this event is part of a different massif than the previous event
				//
				// if it is, we get the new massif
				if massifContext == nil || massifIndex != previousMassifIndex {
					massif, err := cmd.massifReader.GetMassif(cCtx.Context, tenantLogPath, massifIndex)
					if err != nil {
						return err
					}

					massifContext = &massif
				}

				verified, err := event.VerifyInclusion(massifContext)

				// We keep going if the error is a verification failure, as
				// this supports reporting "gaps". All other errors are
				// immediately terminal
				if errors.Is(err, mmr.ErrVerifyInclusionFailed) || !verified {
					countVerifyFailed += 1
					log("XX|%d %d\n", event.MMRIndex(), leafIndex)
					continue
				}

				// all other errors immediately terminal
				if err != nil {
					return err
				}

				proof, err := event.Proof(massifContext)
				if err != nil {
					return err
				}

				log("OK|%d %d|%s", event.MMRIndex(), leafIndex, proofPath(proof))

				previousMassifIndex = massifIndex
			}

			if countVerifyFailed != 0 {
				if len(verifiableLogEntries) == 1 {
					return fmt.Errorf("%w. for tenant %s", ErrVerifyInclusionFailed, tenantIdentity)
				}
				return fmt.Errorf("%w. for tenant %s", ErrVerifyInclusionFailed, tenantIdentity)
			}

			if countNotCommitted > 0 {
				if len(verifiableLogEntries) == 1 {
					return fmt.Errorf("%w. not committed: %d", ErrUncommittedEvents, countNotCommitted)
				}
				return fmt.Errorf("%w. %d events of %d were not committed", ErrUncommittedEvents, countNotCommitted, len(verifiableLogEntries))
			}

			return nil
		},
	}
}
