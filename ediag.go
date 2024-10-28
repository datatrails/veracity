package veracity

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"reflect"
	"time"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/urfave/cli/v2"
)

// NewEventDiagCmd provides diagnostic support for event verification
//
//nolint:gocognit
func NewEventDiagCmd() *cli.Command {
	return &cli.Command{Name: "event-log-info",
		Aliases: []string{"ediag"},
		Usage:   "print diagnostics about an events entry in the log",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "bendump", Aliases: []string{"b"}, Value: false},
			&cli.StringFlag{
				Name: "json", Aliases: []string{"j"},
			},
		},
		Action: func(cCtx *cli.Context) error {
			decodedEvents, err := stdinToDecodedEvents()
			if err != nil {
				return err
			}

			cmd := &CmdCtx{}
			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}
			cmpPrint := func(fmtEq, fmtNe string, a, b any) bool {
				if reflect.DeepEqual(a, b) {
					fmt.Printf(fmtEq, a)
					return true
				}
				fmt.Printf(fmtNe, a, b)
				return false
			}

			for _, decodedEvent := range decodedEvents {

				if decodedEvent.MerkleLog == nil || decodedEvent.MerkleLog.Commit == nil {
					continue
				}

				// Get the mmrIndex from the request and then compute the massif
				// it implies based on the massifHeight command line option.
				mmrIndex := decodedEvent.MerkleLog.Commit.Index
				massifIndex := massifs.MassifIndexFromMMRIndex(cmd.massifHeight, mmrIndex)
				tenantIdentity := cCtx.String("tenant")
				if tenantIdentity == "" {
					// The tenant identity on the event is the original tenant
					// that created the event. For public assets and shared
					// assets, this is true regardless of which tenancy the
					// record is fetched from.  Those same events will appear in
					// the logs of all tenants they were shared with.
					tenantIdentity = decodedEvent.V3Event.TenantIdentity
				}
				// read the massif blob
				cmd.massif, err = cmd.massifReader.GetMassif(context.Background(), tenantIdentity, massifIndex)
				if err != nil {
					return err
				}

				// Get the human time from the idtimestamp committed on the event.

				eventIDTimestamp, _, err := massifs.SplitIDTimestampHex(decodedEvent.MerkleLog.Commit.Idtimestamp)
				if err != nil {
					return err
				}
				eventIDTimestampMS, err := snowflakeid.IDUnixMilli(eventIDTimestamp, uint8(cmd.massif.Start.CommitmentEpoch))
				if err != nil {
					return err
				}

				leafIndex := mmr.LeafIndex(mmrIndex)
				// Note that the banner info is all from the event response
				fmt.Printf("%d %s %s\n", leafIndex, time.UnixMilli(eventIDTimestampMS).Format(time.RFC3339Nano), decodedEvent.V3Event.Identity)

				leafIndexMassif, err := cmd.massif.GetMassifLeafIndex(leafIndex)
				if err != nil {
					return fmt.Errorf("when expecting %d for %d: %v", leafIndexMassif, mmrIndex, err)
				}
				fmt.Printf(" |%8d leaf-index-massif\n", leafIndexMassif)

				// Read the trie entry from the log
				logTrieEntry := massifs.GetTrieEntry(cmd.massif.Data, cmd.massif.IndexStart(), leafIndexMassif)
				logNodeValue, err := cmd.massif.Get(mmrIndex)
				if err != nil {
					return err
				}

				logTrieIDTimestampBytes := logTrieEntry[massifs.TrieEntryIdTimestampStart:massifs.TrieEntryIdTimestampEnd]
				logTrieIDTimestamp := binary.BigEndian.Uint64(logTrieIDTimestampBytes)
				unixMS, err := snowflakeid.IDUnixMilli(logTrieIDTimestamp, uint8(cmd.massif.Start.CommitmentEpoch))
				if err != nil {
					return err
				}
				idTime := time.UnixMilli(unixMS)

				trieKey := massifs.NewTrieKey(
					massifs.KeyTypeApplicationContent,
					[]byte(tenantIdentity),
					[]byte(decodedEvent.V3Event.Identity))
				if len(trieKey) != massifs.TrieKeyBytes {
					return massifs.ErrIndexEntryBadSize
				}
				cmpPrint(
					" |%x trie-key\n",
					" |%x != log-trie-key %x\n", trieKey[:32], logTrieEntry[:32])
				fmt.Printf(" |%x %s log-idtimestamp\n", logTrieIDTimestampBytes, idTime.Format(time.DateTime))
				cmpPrint(
					" |%x idtimestamp\n",
					" |%x != log-idtimestamp %x\n", eventIDTimestamp, logTrieIDTimestamp)

				// Compute the event data hash, independent of domain and idtimestamp

				eventHasher := sha256.New()
				if err = simplehash.V3HashEvent(eventHasher, decodedEvent.V3Event); err != nil {
					return err
				}
				eventHash := eventHasher.Sum(nil)
				fmt.Printf(" |%x v3hash (just the schema fields hashed)\n", eventHash)
				if cCtx.Bool("bendump") {
					bencode, err2 := bencodeEvent(decodedEvent.V3Event)
					if err2 != nil {
						return err2
					}
					fmt.Printf(" |%s\n", string(bencode))
				}

				leafHasher := simplehash.NewHasherV3()
				err = leafHasher.HashEventFromV3(
					decodedEvent.V3Event,
					simplehash.WithPrefix([]byte{LeafTypePlain}),
					simplehash.WithIDCommitted(eventIDTimestamp))
				if err != nil {
					return err
				}
				leafHash := leafHasher.Sum(nil)

				ok := cmpPrint(
					" |%x leaf\n",
					" |%x leaf != log-leaf %x\n", leafHash, logNodeValue)
				if !ok {
					// if the leaf doesn't match we definitely cant verify it
					continue
				}

				// Generate the proof for the mmrIndex and get the root. We use
				// the mmrSize from the end of the blob in which the leaf entry
				// was recorded. Any size > than the leaf index would work.
				mmrSize := cmd.massif.RangeCount()
				proof, err := mmr.InclusionProof(&cmd.massif, mmrSize, mmrIndex)
				if err != nil {
					return err
				}

				verified, err := mmr.VerifyInclusion(&cmd.massif, eventHasher, mmrSize, logNodeValue, mmrIndex, proof)
				if verified {
					fmt.Printf("OK|%d %d\n", mmrIndex, leafIndex)
					continue
				}
				if err != nil {
					fmt.Printf("XX|%d %d|%s\n", mmrIndex, leafIndex, err.Error())
					continue
				}
				fmt.Printf("XX|%d %d\n", mmrIndex, leafIndex)
			}

			return nil
		},
	}
}
