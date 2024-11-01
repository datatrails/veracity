//go:build integration && azurite

package node

import (
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-logverification/integrationsupport"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/go-datatrails-merklelog/mmrtesting"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/datatrails/veracity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *NodeSuite) newMMRTestingConfig(labelPrefix, tenantIdentity string) mmrtesting.TestConfig {
	return mmrtesting.TestConfig{
		StartTimeMS: (1698342521) * 1000, EventRate: 500,
		TestLabelPrefix: labelPrefix,
		TenantIdentity:  tenantIdentity,
		Container:       strings.ReplaceAll(strings.ToLower(labelPrefix), "_", ""),
	}
}

// TestNodeMultiMassif tests that the veracity sub command node
// works for massifs beyond the first one and covers some obvious edge cases.
// This really just tests that the correspondence between the massif index and the leaf index holds
// regardless of the massif count.
func (s *NodeSuite) TestVerifyIncludedMultiMassif() {
	logger.New("TestNodeMultiMassif")
	defer logger.OnExit()

	cfg := s.newMMRTestingConfig("TestNodeMultiMassif", "")
	azurite := mmrtesting.NewTestContext(s.T(), cfg)

	massifHeight := uint8(8)
	leavesPerMassif := mmr.HeightIndexLeafCount(uint64(massifHeight) - 1)

	tests := []struct {
		name        string
		massifCount uint32
		// leaf indices to check.
		leaves []uint64
	}{
		// make sure we cover the obvious edge cases
		{name: "leaf 0", massifCount: 1, leaves: []uint64{0}},
		{name: "single massif first few and last few", massifCount: 1, leaves: []uint64{0, 1, 2, leavesPerMassif - 2, leavesPerMassif - 1}},
		{name: "2 massifs, last of first and first of last", massifCount: 2, leaves: []uint64{leavesPerMassif - 1, leavesPerMassif}},
		{name: "5 massifs, first and last of each", massifCount: 5, leaves: []uint64{
			0, leavesPerMassif - 1,
			1 * leavesPerMassif, 2*leavesPerMassif - 1,
			2 * leavesPerMassif, 3*leavesPerMassif - 1,
			3 * leavesPerMassif, 4*leavesPerMassif - 1,
		}},
	}

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	for _, tt := range tests {

		massifCount := tt.massifCount
		s.Run(fmt.Sprintf("massifCount:%d", massifCount), func() {

			leafHasher := integrationsupport.NewLeafHasher()
			g := integrationsupport.NewTestGenerator(
				s.T(), cfg.StartTimeMS/1000, &leafHasher, mmrtesting.TestGeneratorConfig{
					StartTimeMS:     cfg.StartTimeMS,
					EventRate:       cfg.EventRate,
					TenantIdentity:  cfg.TenantIdentity,
					TestLabelPrefix: cfg.TestLabelPrefix,
				})

			tenantId0 := g.NewTenantIdentity()
			events := integrationsupport.GenerateTenantLog(
				&azurite, g, int(tt.massifCount)*int(leavesPerMassif), tenantId0, true,
				massifHeight,
			)

			for _, iLeaf := range tt.leaves {

				s.ReplaceStdout()

				mmrIndex := mmr.MMRIndex(iLeaf)

				err := app.Run([]string{
					"veracity",
					"--envauth", // uses the emulator
					"--container", cfg.Container,
					"--data-url", s.Env.AzuriteVerifiableDataURL,
					"--tenant", tenantId0,
					"--height", fmt.Sprintf("%d", massifHeight),
					"node",
					"--mmrindex", fmt.Sprintf("%d", mmrIndex),
				})
				s.NoError(err)

				stdout := s.CaptureAndCloseStdout()

				id, _, err := massifs.SplitIDTimestampHex(events[iLeaf].MerklelogEntry.Commit.Idtimestamp)
				require.NoError(s.T(), err)

				hasher := simplehash.NewHasherV3()
				// hash the generated event
				err = hasher.HashEvent(
					events[iLeaf],
					simplehash.WithPrefix([]byte{byte(integrationsupport.LeafTypePlain)}),
					simplehash.WithIDCommitted(id),
				)
				require.Nil(s.T(), err)
				leafValue := fmt.Sprintf("%x", hasher.Sum(nil))
				assert.Equal(s.T(), leafValue, strings.TrimSpace(stdout))
			}
		})
	}
}
