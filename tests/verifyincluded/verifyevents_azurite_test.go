//go:build integration && azurite

package verifyevents

import (
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-logverification/integrationsupport"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/go-datatrails-merklelog/mmrtesting"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/datatrails/veracity"
	"github.com/stretchr/testify/require"
)

func (s *VerifyEventsSuite) newMMRTestingConfig(labelPrefix, tenantIdentity string) mmrtesting.TestConfig {
	return mmrtesting.TestConfig{
		StartTimeMS: (1698342521) * 1000, EventRate: 500,
		TestLabelPrefix: labelPrefix,
		TenantIdentity:  tenantIdentity,
		Container:       strings.ReplaceAll(strings.ToLower(labelPrefix), "_", ""),
	}
}

// TestVerifyIncludedMultiMassif tests that the veracity sub command verify-included
// works for massifs beyond the first one and covers some obvious edge cases.
func (s *VerifyEventsSuite) TestVerifyIncludedMultiMassif() {
	logger.New("TestVerifyIncludedMultiMassif")
	defer logger.OnExit()

	cfg := s.newMMRTestingConfig("TestVerifyIncludedMultiMassif", "")
	azurite := mmrtesting.NewTestContext(s.T(), cfg)

	massifHeight := uint8(8)
	leavesPerMassif := mmr.HeightIndexLeafCount(uint64(massifHeight) - 1)

	tests := []struct {
		name        string
		massifCount uint32
		// leaf indices to verify the inclusion of.
		leaves []uint64
	}{
		// make sure we cover the obvious edge cases
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
				marshaler := simplehash.NewEventMarshaler()
				eventJson, err := marshaler.Marshal(events[iLeaf])
				require.NoError(s.T(), err)
				s.StdinWriteAndClose(eventJson)

				err = app.Run([]string{
					"veracity",
					"--envauth", // uses the emulator
					"--container", cfg.Container,
					"--data-url", s.Env.AzuriteVerifiableDataURL,
					"--tenant", tenantId0,
					"--height", fmt.Sprintf("%d", massifHeight),
					"verify-included",
				})
				s.NoError(err)
				s.ReplaceStdIO() // reset stdin for write & close
			}
		})
	}
}

/*
func (s *VerifyEventsSuite) TestOneTamperEventStdIn() {

	assert := s.Assert()
	app := veracity.NewApp("version", true)
	veracity.AddCommands(app, true)

	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(katdata.KnownTamperedPublicEvent)

	err := app.Run([]string{
		"veracity",
		"--loglevel", "INFO",
		"--tenant", s.Env.PublicTenantId,
		"--data-url", s.Env.VerifiableDataURL,
		"verify-included",
	})
	assert.Errorf(err, "the event should not have verified, its data was purposefully tampered")
}*/
