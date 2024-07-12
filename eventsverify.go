package veracity

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-logverification/logverification"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/urfave/cli/v2"
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

func verifyEvent(event *logverification.VerifiableEvent, massifHeight uint8, massifReader MassifReader, forTenant string) ([][]byte, error) {

	// Get the mmrIndex from the request and then compute the massif
	// it implies based on the massifHeight command line option.
	mmrIndex := event.MerkleLog.Commit.Index

	tenantIdentity := forTenant
	massifIndex := mmr.LeafIndex(mmrIndex+1) / mmr.HeightSize(uint64(massifHeight))
	if tenantIdentity == "" {
		// The tenant identity on the event is the original tenant
		// that created the event. For public assets and shared
		// assets, this is true regardless of which tenancy the
		// record is fetched from.  Those same events will appear in
		// the logs of all tenants they were shared with.
		tenantIdentity = event.TenantID
	}

	// read the massif blob
	massif, err := massifReader.GetMassif(context.Background(), tenantIdentity, massifIndex)
	if err != nil {
		return nil, err
	}

	hasher := sha256.New()
	mmrSize := massif.RangeCount()
	proof, err := mmr.IndexProof(mmrSize, &massif, hasher, mmrIndex)
	if err != nil {
		return nil, err
	}

	hasher.Reset()
	root, err := mmr.GetRoot(mmrSize, &massif, hasher)
	if err != nil {
		return nil, err
	}

	// Note: we verify against the mmrSize of the massif which
	// includes the event. Future work can deepen this to include
	// discovery of the log head, and or verification against a
	// sealed MMRSize.
	hasher.Reset()
	verified := mmr.VerifyInclusion(mmrSize, hasher, event.LeafHash, mmrIndex, proof, root)
	if verified {
		return proof, nil

	}

	return nil, fmt.Errorf("event not included")
}

// NewEventsVerifyCmd verifies inclusion of a DataTrails event in the tenants Merkle Log
//
//nolint:gocognit
func NewEventsVerifyCmd() *cli.Command {
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

			log("verifying events dir: %s", cCtx.String("logdir"))

			verifiableEvents, err := readArgs0FileOrStdIoToVerifiableEvent(cCtx)
			if err != nil {
				return err
			}

			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}

			tenantIdentity := cCtx.String("tenant")
			if tenantIdentity != "" {
				log("verifying for tenant: %s", tenantIdentity)
			} else {
				log("verifying protected events for the asset creator")
			}

			var countNotCommitted int
			var countVerifyFailed int

			for _, event := range verifiableEvents {

				// don't try if we don't even have any merkle log entries on this event
				if event.MerkleLog == nil || event.MerkleLog.Commit == nil {
					countNotCommitted += 1
					log("not committed: %s", event.EventID)
					continue
				}

				mmrIndex := event.MerkleLog.Commit.Index
				leafIndex := mmr.LeafIndex(mmrIndex)
				log("verifying: %d %d %s %s", mmrIndex, leafIndex, event.MerkleLog.Commit.Idtimestamp, event.EventID)
				proof, err := verifyEvent(&event, cmd.massifHeight, cmd.massifReader, tenantIdentity)
				if err != nil {
					countVerifyFailed += 1
					log("XX|%d %d\n", mmrIndex, leafIndex)
					continue
				}

				log("OK|%d %d|%s", mmrIndex, leafIndex, proofPath(proof))
			}

			if countVerifyFailed != 0 {
				if len(verifiableEvents) == 1 {
					return fmt.Errorf("%w. for tenant %s", ErrVerifyInclusionFailed, tenantIdentity)
				}
				return fmt.Errorf("%w. for tenant %s", ErrVerifyInclusionFailed, tenantIdentity)
			}

			if countNotCommitted > 0 {
				if len(verifiableEvents) == 1 {
					return fmt.Errorf("%w. not committed: %d", ErrUncommittedEvents, countNotCommitted)
				}
				return fmt.Errorf("%w. %d events of %d were not committed", ErrUncommittedEvents, countNotCommitted, len(verifiableEvents))
			}

			return nil
		},
	}
}
