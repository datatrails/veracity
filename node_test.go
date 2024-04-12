//go:build integration && azurite

package veracity

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	merklelogmmrblobs "github.com/datatrails/forestrie/go-forestrie/merklelog/mmrblobs"
	"github.com/datatrails/forestrie/go-forestrie/mmrtesting"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/stretchr/testify/assert"
)

const (
	massifHeight = 14
)

// TestConfigReaderForEmulator tests that we can run the veracity tool against
// emulator urls.
func TestNodeCmd(t *testing.T) {
	logger.New("TestVerifyList")
	defer logger.OnExit()
	url := os.Getenv("TEST_INTEGRATION_FORESTRIE_BLOBSTORE_URL")
	logger.Sugar.Infof("url: '%s'", url)

	// Create a single massif in the emulator

	tenantID := mmrtesting.DefaultGeneratorTenantIdentity
	testContext, testGenerator, cfg := merklelogmmrblobs.NewAzuriteTestContext(t, "TestNodeCmd")
	merklelogmmrblobs.GenerateTenantLog(&testContext, testGenerator, 10, tenantID, true)

	tests := []struct {
		testArgs []string
	}{
		// get node 1
		{testArgs: []string{"<progname>", "-s", "devstoreaccount1", "-c", cfg.Container, "-t", tenantID, "node", fmt.Sprintf("%d", 1)}},
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
