//go:build integration && azurite

package veracity

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datatrails/forestrie/go-forestrie/massifs"
	"github.com/datatrails/forestrie/go-forestrie/merklelog"
	merklelogmmrblobs "github.com/datatrails/forestrie/go-forestrie/merklelog/mmrblobs"
	"github.com/datatrails/forestrie/go-forestrie/mmr"
	"github.com/datatrails/forestrie/go-forestrie/mmrtesting"
	v2assets "github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigReaderForEmulator tests that we can run the veracity tool against
// emulator urls.
func TestNodeScanCmd(t *testing.T) {
	logger.New("TestVerifyList")
	defer logger.OnExit()
	url := os.Getenv("TEST_INTEGRATION_FORESTRIE_BLOBSTORE_URL")
	logger.Sugar.Infof("url: '%s'", url)

	// Create a single massif in the emulator

	tenantID := mmrtesting.DefaultGeneratorTenantIdentity
	testContext, testGenerator, cfg := merklelogmmrblobs.NewAzuriteTestContext(t, "TestNodeScanCmd")

	eventsResponse := merklelogmmrblobs.GenerateTenantLog(&testContext, testGenerator, 10, tenantID, true, massifHeight)
	marshaledEvents, err := marshalEventsList(eventsResponse)
	require.NoError(t, err)

	// Arbitrarily chose to look for leaf 7
	leafIndex := 7

	// We could try an capture the output from the tool and check this, but that
	// sort of thing is extensively checked elsewhere. Here we are just
	// confirming the derived leaf hash is matched in the blob. WHich is the
	// purpose of the diagnostic tool. But we log the expected mmrIndex for development ease.

	fmt.Printf("TreeIndex(%d) == %d\n", leafIndex, mmr.TreeIndex(uint64(leafIndex)))

	// Get the idtimestamp that ensures every entry is unique
	idTimestamp, _, err := massifs.SplitIDTimestampHex(
		eventsResponse[leafIndex].MerklelogEntry.Commit.Idtimestamp)
	require.NoError(t, err)

	// Produce the expected leaf hash for the tree entry for this event
	simplehashv3Hasher := simplehash.NewHasherV3()
	err = simplehashv3Hasher.HashEventFromJSON(
		marshaledEvents[leafIndex],
		simplehash.WithPrefix([]byte{uint8(merklelog.LeafTypePlain)}),
		simplehash.WithIDCommitted(idTimestamp))
	require.NoError(t, err)
	expectedLeafNodeValue := fmt.Sprintf("%x", simplehashv3Hasher.Sum(nil))

	tests := []struct {
		testArgs []string
	}{
		// match the expected leaf hash by scanning massif 0. typically this is
		// useful only as a diagnostic tool or triage tool. typically the
		// precise node is located by the mmrIndex and the leafIndex derived
		// from that.
		{testArgs: []string{
			"<progname>", "-s", "devstoreaccount1", "-c", cfg.Container, "-t", tenantID,
			"nodescan", "-m", "0", "-v", expectedLeafNodeValue}},
	}

	for _, tc := range tests {
		t.Run(strings.Join(tc.testArgs, " "), func(t *testing.T) {
			app := AddCommands(NewApp())
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			err := app.RunContext(ctx, tc.testArgs)
			cancel()
			assert.NoError(t, err)
		})
	}
}

func marshalEventsList(eventsResponse []*v2assets.EventResponse) ([][]byte, error) {
	marshaller := v2assets.NewFlatMarshalerForEvents()

	eventJsonList := make([][]byte, 0)
	for _, event := range eventsResponse {
		eventJson, err := marshaller.Marshal(event)
		if err != nil {
			return nil, err
		}

		eventJsonList = append(eventJsonList, eventJson)
	}
	return eventJsonList, nil
}
