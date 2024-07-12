package veracity

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
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

// NewEventsVerifyCmd verifies inclusion of a DataTrails event in the tenants Merkle Log
//
//nolint:gocognit
func NewEventsVerifyCmd() *cli.Command {
	return &cli.Command{
		Name:    "events-verify",
		Aliases: []string{"everify"},
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

			verifiableEvents, err := stdinToVerifiableEvents()
			if err != nil {
				return err
			}

			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}

			proofPath := func(proof [][]byte) string {
				var hexProof []string
				for _, node := range proof {
					hexProof = append(hexProof, fmt.Sprintf("%x", node))
				}
				return fmt.Sprintf("[%s]", strings.Join(hexProof, ", "))
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

				if event.MerkleLog == nil || event.MerkleLog.Commit == nil {
					countNotCommitted += 1
					log("not committed: %s", event.EventID)
					continue
				}

				// Get the mmrIndex from the request and then compute the massif
				// it implies based on the massifHeight command line option.
				mmrIndex := event.MerkleLog.Commit.Index

				massifIndex := mmr.LeafIndex(mmrIndex+1) / mmr.HeightSize(uint64(cmd.massifHeight))
				if tenantIdentity == "" {
					// The tenant identity on the event is the original tenant
					// that created the event. For public assets and shared
					// assets, this is true regardless of which tenancy the
					// record is fetched from.  Those same events will appear in
					// the logs of all tenants they were shared with.
					tenantIdentity = event.TenantID
				}

				// read the massif blob
				cmd.massif, err = cmd.massifReader.GetMassif(context.Background(), tenantIdentity, massifIndex)
				if err != nil {
					return err
				}

				eventIDTimestamp, _, err := massifs.SplitIDTimestampHex(event.MerkleLog.Commit.Idtimestamp)
				if err != nil {
					return err
				}
				// Get the human time from the idtimestamp committed on the event for the telemetry
				eventIDTimestampMS, err := snowflakeid.IDUnixMilli(eventIDTimestamp, uint8(cmd.massif.Start.CommitmentEpoch))
				if err != nil {
					return err
				}

				leafIndex := mmr.LeafIndex(mmrIndex)

				log("verifying: %d %d %s %s %s", mmrIndex, leafIndex, event.MerkleLog.Commit.Idtimestamp, time.UnixMilli(eventIDTimestampMS).Format(time.RFC3339Nano), event.EventID)

				hasher := sha256.New()
				mmrSize := cmd.massif.RangeCount()
				proof, err := mmr.IndexProof(mmrSize, &cmd.massif, hasher, mmrIndex)
				if err != nil {
					return err
				}

				hasher.Reset()
				root, err := mmr.GetRoot(mmrSize, &cmd.massif, hasher)
				if err != nil {
					return err
				}

				// Note: we verify against the mmrSize of the massif which
				// includes the event. Future work can deepen this to include
				// discovery of the log head, and or verification against a
				// sealed MMRSize.
				hasher.Reset()
				verified := mmr.VerifyInclusion(mmrSize, hasher, event.LeafHash, mmrIndex, proof, root)
				if verified {
					log("OK|%d %d|%s", mmrIndex, leafIndex, proofPath(proof))
					continue
				}

				countVerifyFailed += 1
				log("XX|%d %d\n", mmrIndex, leafIndex)
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
