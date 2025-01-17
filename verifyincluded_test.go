package veracity

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-logverification/logverification/app"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	veracityapp "github.com/datatrails/veracity/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMassifContext generates a massif context with 2 entries
//
// the first entry is a known assetsv2 events
// the seconds entry is a known eventsv1 event
func testMassifContext(t *testing.T) *massifs.MassifContext {

	start := massifs.MassifStart{
		MassifHeight: 3,
	}

	testMassifContext := &massifs.MassifContext{
		Start: start,
		LogBlobContext: massifs.LogBlobContext{
			BlobPath: "test",
			Tags:     map[string]string{},
		},
	}

	data, err := start.MarshalBinary()
	require.NoError(t, err)

	testMassifContext.Data = append(data, testMassifContext.InitIndexData()...)

	testMassifContext.Tags["firstindex"] = fmt.Sprintf("%016x", testMassifContext.Start.FirstIndex)

	hasher := sha256.New()

	// KAT Data taken from an actual merklelog.

	// AssetsV2
	_, err = testMassifContext.AddHashedLeaf(
		hasher,
		binary.BigEndian.Uint64([]byte{148, 111, 227, 95, 198, 1, 121, 0}),
		[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]byte("tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0"),
		[]byte("assets/899e00a2-29bc-4316-bf70-121ce2044472/events/450dce94-065e-4f6a-bf69-7b59f28716b6"),
		[]byte{97, 231, 1, 42, 127, 20, 181, 70, 122, 134, 84, 231, 174, 117, 200, 148, 171, 205, 57, 146, 174, 48, 34, 30, 152, 215, 77, 3, 204, 14, 202, 57},
	)
	require.NoError(t, err)

	// EventsV1
	_, err = testMassifContext.AddHashedLeaf(
		hasher,
		binary.BigEndian.Uint64([]byte{148, 112, 0, 54, 17, 1, 121, 0}),
		[]byte{1, 17, 39, 88, 206, 168, 203, 73, 36, 141, 248, 252, 186, 30, 49, 248, 176, 0, 0, 0, 0, 0, 0, 0},
		[]byte("tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0"),
		[]byte("events/01947000-3456-780f-bfa9-29881e3bac88"),
		[]byte{215, 191, 107, 210, 134, 10, 40, 56, 226, 71, 136, 164, 9, 118, 166, 159, 86, 31, 175, 135, 202, 115, 37, 151, 174, 118, 115, 113, 25, 16, 144, 250},
	)
	require.NoError(t, err)

	// Intermediate Node Skipped

	return testMassifContext
}

type fakeMassifGetter struct {
	t             *testing.T
	massifContext *massifs.MassifContext
}

// NewFakeMassifGetter creates a new massif getter that has 2 entries in the massif it gets
//
// one assetsv2 event entry and one eventsv1 entry
func NewFakeMassifGetter(t *testing.T) *fakeMassifGetter {

	massifContext := testMassifContext(t)

	return &fakeMassifGetter{
		t:             t,
		massifContext: massifContext,
	}

}

// NewFakeMassifGetterInvalidRoot creates a new massif getter that has an incorrect massif root
func NewFakeMassifGetterInvalidRoot(t *testing.T) *fakeMassifGetter {

	massifContext := testMassifContext(t)

	// a massif context with 2 entries has its root at index 2
	//
	//   2
	//  / \
	// 0   1
	rootMMRIndex := 2

	rootDataStart := (massifContext.LogStart() + uint64(rootMMRIndex*massifs.LogEntryBytes)) - 1
	rootDataEnd := (rootDataStart + massifs.ValueBytes)

	// set the start and end of the root entry to 0
	//  to make the root entry invalid
	massifContext.Data[rootDataStart] = 0x0
	massifContext.Data[rootDataEnd] = 0x0

	return &fakeMassifGetter{
		t:             t,
		massifContext: massifContext,
	}
}

// GetMassif always returns the test massif
func (tmg *fakeMassifGetter) GetMassif(
	ctx context.Context,
	tenantIdentity string,
	massifIndex uint64,
	opts ...massifs.ReaderOption,
) (massifs.MassifContext, error) {
	return *tmg.massifContext, nil
}

func TestVerifyAssetsV2Event(t *testing.T) {
	logger.New("TestVerifyList")
	defer logger.OnExit()

	events, _ := veracityapp.NewAssetsV2AppEntries(assetsV2SingleEventList)
	require.NotZero(t, len(events))

	event := events[0]

	tests := []struct {
		name          string
		event         *app.AppEntry
		massifGetter  MassifGetter
		expectedProof [][]byte
		expectedError bool
	}{
		{
			name:          "simple OK",
			event:         &event,
			massifGetter:  NewFakeMassifGetter(t),
			expectedError: false,
			expectedProof: [][]byte{
				{
					0xd7, 0xbf, 0x6b, 0xd2, 0x86, 0xa, 0x28, 0x38,
					0xe2, 0x47, 0x88, 0xa4, 0x9, 0x76, 0xa6, 0x9f,
					0x56, 0x1f, 0xaf, 0x87, 0xca, 0x73, 0x25, 0x97,
					0xae, 0x76, 0x73, 0x71, 0x19, 0x10, 0x90, 0xfa,
				},
			},
		},
		/**{
			name:          "No mmr log",
			event:         &event,
			massifGetter:  &fakeMassifGetter{t, nil},
			expectedError: true,
		},*/
		{
			name:          "Not valid proof",
			event:         &event,
			massifGetter:  NewFakeMassifGetterInvalidRoot(t),
			expectedError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			logTenant, err := test.event.LogTenant()
			require.Nil(t, err)

			massifIndex := massifs.MassifIndexFromMMRIndex(defaultMassifHeight, test.event.MMRIndex())

			ctx := context.Background()
			massif, err := test.massifGetter.GetMassif(ctx, logTenant, massifIndex)
			require.NoError(t, err)

			mmrEntry, err := test.event.MMREntry(&massif)
			require.NoError(t, err)

			proof, err := verifyEvent(test.event, logTenant, mmrEntry, defaultMassifHeight, test.massifGetter)

			if test.expectedError {
				assert.NotNil(t, err, "expected error got nil")
			} else {
				assert.Nil(t, err, "unexpected error")
				assert.Equal(t, test.expectedProof, proof)
			}
		})
	}
}
