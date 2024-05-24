package veracitytesting

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
	"github.com/datatrails/go-datatrails-common-api-gen/attribute/v2/attribute"
	"github.com/datatrails/go-datatrails-common/azbus"
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"github.com/datatrails/go-datatrails-merklelog/mmrtesting"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	resourceChangedProperty            = "resource_changed"
	resourceChangeMerkleLogStoredEvent = "assetsv2merklelogeventstored"
)

type leafHasher interface {
	Reset()
	Sum(b []byte) []byte
	HashEvent(event *assets.EventResponse, opts ...simplehash.HashOption) error
}

// Create random values of various sorts for testing. Seeded so that from run to
// run the values are the same. Intended for white box tests that benefit from a
// large volume of synthetic data.
type EventTestGenerator struct {
	mmrtesting.TestGenerator
	numEventsGenerated int
	LeafHasher         leafHasher
	IdState            *snowflakeid.IDState
}

func NewAzuriteTestContext(
	t *testing.T,
	testLabelPrefix string,
) (mmrtesting.TestContext, EventTestGenerator, mmrtesting.TestConfig) {
	cfg := mmrtesting.TestConfig{
		StartTimeMS: (1698342521) * 1000, EventRate: 500,
		TestLabelPrefix: testLabelPrefix,
		TenantIdentity:  "",
		Container:       strings.ReplaceAll(strings.ToLower(testLabelPrefix), "_", "")}
	leafHasher := NewLeafHasher()
	tc := mmrtesting.NewTestContext(t, cfg)
	g := NewEventTestGenerator(
		t, cfg.StartTimeMS/1000,
		&leafHasher,
		mmrtesting.TestGeneratorConfig{
			StartTimeMS:     cfg.StartTimeMS,
			EventRate:       cfg.EventRate,
			TenantIdentity:  cfg.TenantIdentity,
			TestLabelPrefix: cfg.TestLabelPrefix,
		},
	)
	return tc, g, cfg
}

// NewTestGenerator creates a deterministic, but random looking, test data generator.
// Given the same seed, the series of data generated on different runs is identical.
// This means that we generate valid values for things like uuid based
// identities and simulated time stamps, but the log telemetry from successive runs will
// be usefuly stable.
func NewEventTestGenerator(
	t *testing.T, seed int64,
	leafHasher leafHasher,
	cfg mmrtesting.TestGeneratorConfig) EventTestGenerator {

	g := EventTestGenerator{
		LeafHasher: leafHasher,
	}
	g.TestGenerator = mmrtesting.NewTestGenerator(t, seed, cfg, func(tenantIdentity string, base, i uint64) mmrtesting.AddLeafArgs {
		return g.GenerateLeaf(tenantIdentity, base, i)
	})

	var err error
	g.IdState, err = snowflakeid.NewIDState(snowflakeid.Config{
		CommitmentEpoch: 1,
		WorkerCIDR:      "0.0.0.0/16",
		PodIP:           "10.0.0.1",
	})
	require.NoError(t, err)

	return g
}

func (g *EventTestGenerator) NextId() (uint64, error) {
	var err error
	var id uint64

	for i := 0; i < 2; i++ {
		id, err = g.IdState.NextID()
		if err != nil {
			if !errors.Is(err, snowflakeid.ErrOverloaded) {
				return 0, err
			}
			time.Sleep(time.Millisecond * 2)
		}
	}
	return id, nil
}

func (g *EventTestGenerator) GenerateLeaf(tenantIdentity string, base, i uint64) mmrtesting.AddLeafArgs {
	ev := g.GenerateNextEvent(tenantIdentity)

	id, err := g.NextId()
	require.NoError(g.T, err)
	g.LeafHasher.Reset()
	err = g.LeafHasher.HashEvent(ev)
	require.Nil(g.T, err)

	return mmrtesting.AddLeafArgs{
		Id:    id,
		AppId: []byte(ev.GetIdentity()),
		Value: g.LeafHasher.Sum(nil),
	}
}

func (g *EventTestGenerator) GenerateEventBatch(count int) []*assets.EventResponse {
	events := make([]*assets.EventResponse, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, g.GenerateNextEvent(mmrtesting.DefaultGeneratorTenantIdentity))
	}
	return events
}

func (g *EventTestGenerator) GenerateTenantEventMessageBatch(tenantIdentity string, count int) []*azbus.ReceivedMessage {
	msgs := make([]*azbus.ReceivedMessage, 0, count)
	for i := 0; i < count; i++ {
		event := assets.EventMessage{
			TenantId: tenantIdentity,
			Event:    g.GenerateNextEvent(tenantIdentity),
		}
		m := protojson.MarshalOptions{UseProtoNames: true}
		data, err := m.Marshal(&event)
		if err != nil {
			g.T.Fatalf("failed to marshal event: %v", err)
		}

		props := make(map[string]any)
		props[resourceChangedProperty] = resourceChangeMerkleLogStoredEvent

		msgs = append(msgs, &azbus.ReceivedMessage{Body: data, ApplicationProperties: props})
	}
	return msgs
}

func (g *EventTestGenerator) GenerateNextEvent(tenantIdentity string) *assets.EventResponse {

	assetIdentity := g.NewAssetIdentity()
	assetUUID := strings.Split(assetIdentity, "/")[1]

	name := strings.Join(g.WordList(2), "")
	email := fmt.Sprintf("%s@datatrails.com", name)
	subject := strconv.Itoa(g.Intn(math.MaxInt))

	// Use the desired event rate as the upper bound, and generate a time stamp at lastTime + rand(0, upper-bound * 2)
	// So the generated event stream will be around the target rate.
	ts := g.SinceLastJitter()

	event := &assets.EventResponse{
		Identity:      g.NewEventIdentity(assetUUID),
		AssetIdentity: assetIdentity,
		EventAttributes: map[string]*attribute.Attribute{
			"forestrie.testGenerator-sequence-number": {
				Value: &attribute.Attribute_StrVal{
					StrVal: strconv.Itoa(g.numEventsGenerated),
				},
			},
			"forestrie.testGenerator-label": {
				Value: &attribute.Attribute_StrVal{
					StrVal: fmt.Sprintf("%s%s", g.Cfg.TestLabelPrefix, "GenerateNextEvent"),
				},
			},

			"event-attribute-0": {
				Value: &attribute.Attribute_StrVal{
					StrVal: g.MultiWordString(6),
				},
			},
		},
		AssetAttributes: map[string]*attribute.Attribute{
			"asset-attribute-0": {
				Value: &attribute.Attribute_StrVal{
					StrVal: g.MultiWordString(4),
				},
			},
		},
		Operation:          "Record",
		Behaviour:          "RecordEvidence",
		TimestampDeclared:  timestamppb.New(ts),
		TimestampAccepted:  timestamppb.New(ts),
		TimestampCommitted: nil,
		PrincipalDeclared: &assets.Principal{
			Issuer:      "https://rkvt.com",
			Subject:     subject,
			DisplayName: name,
			Email:       email,
		},
		PrincipalAccepted: &assets.Principal{
			Issuer:      "https://rkvt.com",
			Subject:     subject,
			DisplayName: name,
			Email:       email,
		},
		ConfirmationStatus: assets.ConfirmationStatus_PENDING,
		From:               "0xf8dfc073650503aeD429E414bE7e972f8F095e70",
		// TenantIdentity:     "tenant/0684984b-654d-4301-ad10-a508126e187d",
		TenantIdentity: tenantIdentity,
	}
	g.LastTime = ts
	g.numEventsGenerated++

	return event
}

func (g *EventTestGenerator) NewEventIdentity(assetUUID string) string {
	return assets.EventIdentityFromUuid(assetUUID, g.NewRandomUUIDString(g.T))
}

func (g *EventTestGenerator) NewAssetIdentity() string {
	return assets.AssetIdentityFromUuid(g.NewRandomUUIDString(g.T))
}

// PadWithLeafEntries pads the given mmr (data) with the given number of leaves (n).
//
//	Each leaf is a hash of a deterministically generated event.
func (g *EventTestGenerator) PadWithLeafEntries(data []byte, n int) []byte {
	if n == 0 {
		return data
	}
	g.LeafHasher.Reset()
	g.LeafHasher.Reset()

	batch := g.GenerateEventBatch(n)
	for _, ev := range batch {
		err := g.LeafHasher.HashEvent(ev)
		require.NoError(g.T, err)
		v := g.LeafHasher.Sum(nil)
		data = append(data, v...)
	}
	return data
}
