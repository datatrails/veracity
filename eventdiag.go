package veracity

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"reflect"
	"time"

	v2assets "github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LeafType provides domain separation for the different kinds of tree leaves we require.
type LeafType uint8

const (
	// LeafTypePlain is used for committing to plain values.
	LeafTypePlain LeafType = iota
	// LeafTypePeriodSentinel is entered into the MMR once per period. By
	// forcing a heartbeat entry, we guarantee a liveness indicator - their will
	// be a definable lower bound on how often the MMR root changes
	LeafTypePeriodSentinel
	// LeafTypeEpochTombstone is always the last leave in an epoch MMR. This is
	// used to provide crash fault tolerance on the epoch as whole
	LeafTypeEpochTombStone
)

func NewTimestamp(id uint64, epoch uint8) (*timestamppb.Timestamp, error) {
	ts := &timestamppb.Timestamp{}
	err := SetTimestamp(id, ts, epoch)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func SetTimestamp(id uint64, ts *timestamppb.Timestamp, epoch uint8) error {
	ms, err := snowflakeid.IDUnixMilli(id, epoch)
	if err != nil {
		return err
	}

	ts.Seconds = ms / 1000
	ts.Nanos = int32(uint64(ms)-(uint64(ts.GetSeconds())*1000)) * 1e6

	return nil
}

// NewEventDiagCmd provides diagnostic support for event verification
//
//nolint:gocognit
func NewEventDiagCmd() *cli.Command {
	return &cli.Command{Name: "ediag",
		Usage: "print diagnostics about an events entry in the log",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "bendump", Aliases: []string{"b"}, Value: false},
			&cli.StringFlag{
				Name: "json", Aliases: []string{"j"},
			},
		},
		Action: func(cCtx *cli.Context) error {
			eventsJson, err := readFile(cCtx)
			if err != nil {
				return err
			}

			cmd := &CmdCtx{}
			if err = cfgBugs(cmd, cCtx); err != nil {
				return err
			}
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

			for _, eventJson := range eventsJson {

				resp := v2assets.EventResponseJSONAPI{}
				err = protojson.Unmarshal(eventJson, &resp)
				if err != nil {
					return err
				}

				// If we are compensating for bug 9308, do so by restoring timestamp_committed from the idtimestamp
				if Bug(cmd, Bug9308) {
					v3, err := simplehash.V3FromEventJSON(eventJson)
					if err != nil {
						return err
					}

					event, err := newEventResponseFromV3(&v3)
					if err != nil {
						return err
					}

					id, epoch, err := massifs.SplitIDTimestampHex(resp.MerklelogEntry.Commit.Idtimestamp)
					if err != nil {
						return err
					}

					event.TimestampCommitted, err = NewTimestamp(id, epoch)
					if err != nil {
						return err
					}

					// due to https://dev.azure.com/jitsuin/avid/_workitems/edit/9323
					// We have to go via the besboke marshaler

					marshaler := v2assets.NewFlatMarshalerForEvents()
					eventJson, err = marshaler.Marshal(event)
					if err != nil {
						return err
					}

					respRecovered := v2assets.EventResponseJSONAPI{}
					err = protojson.Unmarshal(eventJson, &respRecovered)
					if err != nil {
						return err
					}

					// ms := event.TimestampCommitted.AsTime().UnixMilli()
					// fmt.Printf("%d %s %s\n", ms, event.TimestampCommitted.AsTime().Format(time.RFC3339Nano), respRecovered.TimestampCommitted)
				}

				if resp.MerklelogEntry == nil || resp.MerklelogEntry.Commit == nil {
					continue
				}

				// Get the mmrIndex from the request and then compute the massif
				// it implies based on the massifHeight command line option.
				mmrIndex := resp.MerklelogEntry.Commit.Index
				massifIndex, err := massifs.MassifIndexFromMMRIndex(cmd.massifHeight, mmrIndex)
				if err != nil {
					return err
				}
				// read the massif blob
				cmd.massif, err = cmd.massifReader.GetMassif(context.Background(), resp.TenantIdentity, massifIndex)
				if err != nil {
					return err
				}

				// Get the human time from the idtimestamp committed on the event.

				// the idCommitted is in hex from the event, we need to convert it to uint64
				// idCommitted, _, err := massifs.SplitIDTimestampHex(merkleLogEntry.Commit.Idtimestamp)
				// if err != nil {
				// 	return err
				// }
				respIdTimestamp, _, err := massifs.SplitIDTimestampHex(resp.MerklelogEntry.Commit.Idtimestamp)
				if err != nil {
					return err
				}
				respIDTimeMS, err := snowflakeid.IDUnixMilli(respIdTimestamp, uint8(cmd.massif.Start.CommitmentEpoch))
				if err != nil {
					return err
				}

				leafIndex := mmr.LeafCount(mmrIndex)
				// Note that the banner info is all from the event response
				fmt.Printf("%d %s %s\n", leafIndex, time.UnixMilli(respIDTimeMS).Format(time.RFC3339Nano), resp.Identity)

				leafIndexMassif, err := cmd.massif.GetMassifLeafIndex(leafIndex)
				if err != nil {
					return fmt.Errorf("when expecting %d for %d: %v", leafIndexMassif, mmrIndex, err)
				}
				fmt.Printf(" |%8d leaf-index-massif\n", leafIndexMassif)

				// Read the trie entry from the log
				logTrieKey := massifs.GetTrieEntry(cmd.massif.Data, cmd.massif.IndexStart(), leafIndexMassif)
				logNodeValue, err := cmd.massif.Get(mmrIndex)
				if err != nil {
					return err
				}

				trieKeyIDBytes := logTrieKey[massifs.TrieEntryIdTimestampStart:massifs.TrieEntryIdTimestampEnd]
				trieKeyID := binary.BigEndian.Uint64(trieKeyIDBytes)
				unixMS, err := snowflakeid.IDUnixMilli(trieKeyID, uint8(cmd.massif.Start.CommitmentEpoch))
				if err != nil {
					return err
				}
				idTime := time.UnixMilli(unixMS)

				// Encode the api response in the V3 Schema
				v3Event, err := simplehash.V3FromEventJSON(eventJson)
				if err != nil {
					return err
				}

				trieKey := massifs.NewTrieKey(massifs.KeyTypeApplicationContent, []byte(v3Event.TenantIdentity), []byte(v3Event.Identity))
				if len(trieKey) != massifs.TrieKeyBytes {
					return massifs.ErrIndexEntryBadSize
				}
				cmpPrint(
					" |%x trie-key\n",
					" |%x != log-trie-key %x\n", trieKey[:32], logTrieKey[:32])
				fmt.Printf(" |%x %s log-trie-key-id\n", trieKeyIDBytes, idTime.Format(time.DateTime))

				simplehashv3Hasher := simplehash.NewHasherV3()

				// Compute the event data hash, independent of domain and idtimestamp

				hasher := sha256.New()
				if err = simplehash.V3HashEvent(hasher, v3Event); err != nil {
					return err
				}
				eventHash := hasher.Sum(nil)

				err = simplehashv3Hasher.HashEventFromJSON(
					eventJson,
					simplehash.WithPrefix([]byte{uint8(LeafTypePlain)}),
					simplehash.WithIDCommitted(respIdTimestamp))

				if err != nil {
					return err
				}
				fmt.Printf(" |%x v3hash (just the schema fields hashed)\n", eventHash)
				if cCtx.Bool("bendump") {
					bencode, err2 := bencodeEvent(v3Event)
					if err2 != nil {
						return err2
					}
					fmt.Printf(" |%s\n", string(bencode))
				}

				ok := cmpPrint(
					" |%x leaf\n",
					" |%x leaf != log-leaf %x\n", simplehashv3Hasher.Sum(nil), logNodeValue)
				if !ok {
					// if the leaf doesn't match we definitely cant verify it
					continue
				}

				// Generate the proof for the mmrIndex and get the root. We use
				// the mmrSize from the end of the blob in which the leaf entry
				// was recorded. Any size > than the leaf index would work.
				hasher.Reset()
				mmrSize := cmd.massif.RangeCount()
				proof, err := mmr.IndexProof(mmrSize, &cmd.massif, hasher, mmrIndex)
				if err != nil {
					return err
				}
				root, err := mmr.GetRoot(mmrSize, &cmd.massif, hasher)
				if err != nil {
					return err
				}

				hasher.Reset()
				verified := mmr.VerifyInclusion(mmrSize, hasher, logNodeValue, mmrIndex, proof, root)
				if verified {
					fmt.Printf("OK|%d %d\n", mmrIndex, leafIndex)
					continue
				}
				fmt.Printf("XX|%d %d\n", mmrIndex, leafIndex)
			}

			return nil
		},
	}
}
