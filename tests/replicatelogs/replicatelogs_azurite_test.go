//go:build integration && azurite

package verifyconsistency

import (
	"context"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/veracity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fileSHA256(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

func mustHashFile(t *testing.T, filename string) []byte {
	t.Helper()
	hash, err := fileSHA256(filename)
	require.NoError(t, err)
	return hash
}

// TestReplicateMassifUpdate ensures that an extension to a previously replicated
// massif is handled correctly
func (s *ReplicateLogsCmdSuite) TestReplicateMassifUpdate() {
	logger.New("TestReplicateMassifUpdate")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestReplicateMassifUpdate")

	h8MassifLeaves := mmr.HeightIndexLeafCount(uint64(8 - 1)) // = ((2 << massifHeight) - 1 + 1) >> 1

	tests := []struct {
		name                   string
		massifHeight           uint8
		firstUpdateMassifs     uint64
		firstUpdateExtraLeaves uint64
		// if zero, the last massif will be completed. If the last is full and secondUpdateMassifs is zero the test is invalid
		secondUpdateMassifs     uint64
		secondUpdateExtraLeaves uint64
	}{
		// extend second massif
		{name: "complete first massif", massifHeight: 8, firstUpdateMassifs: 1, firstUpdateExtraLeaves: h8MassifLeaves - 3, secondUpdateMassifs: 0, secondUpdateExtraLeaves: 3},

		// make sure we cover the obvious edge cases
		{name: "complete first massif", massifHeight: 8, firstUpdateMassifs: 0, firstUpdateExtraLeaves: h8MassifLeaves - 3, secondUpdateMassifs: 0, secondUpdateExtraLeaves: 3},

		// make sure we cover update from partial blob to new massif
		{name: "partial first massif", massifHeight: 8, firstUpdateMassifs: 0, firstUpdateExtraLeaves: h8MassifLeaves - 6, secondUpdateMassifs: 2, secondUpdateExtraLeaves: 0},
	}
	key := massifs.TestGenerateECKey(s.T(), elliptic.P256())

	for _, tt := range tests {

		s.Run(tt.name, func() {

			// Populate the log with the content for the first update

			require.True(s.T(), tt.firstUpdateMassifs > 0 || tt.firstUpdateExtraLeaves > 0, uint32(0), "invalid test")
			require.True(s.T(), tt.secondUpdateMassifs > 0 || tt.secondUpdateExtraLeaves > 0, uint32(0), "invalid test")
			replicaDir := s.T().TempDir()
			tenantId0 := tc.G.NewTenantIdentity()

			// If we skip CreateLog below, we need to delete the blobs
			tc.AzuriteContext.DeleteBlobsByPrefix(massifs.TenantMassifPrefix(tenantId0))

			if tt.firstUpdateMassifs > 0 {
				tc.CreateLog(
					tenantId0, tt.massifHeight, uint32(tt.firstUpdateMassifs),
					massifs.TestWithSealKey(&key),
				)
			}
			if tt.firstUpdateExtraLeaves > 0 {
				tc.AddLeavesToLog(
					tenantId0, tt.massifHeight, int(tt.firstUpdateExtraLeaves),
					massifs.TestWithSealKey(&key),
				)
			}

			// Replicate the log
			// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
			app := veracity.NewApp("tests", true)
			veracity.AddCommands(app, true)

			err := app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", tt.massifHeight),
				"replicate-logs",
				// "--ancestors", fmt.Sprintf("%d", tt.ancestors),
				"--replicadir", replicaDir,
				// note: firstUpdateMassifs is the count of *full* massifs we add before adding the leaves, so the count is also the "index" of the last massif.
				"--massif", fmt.Sprintf("%d", tt.firstUpdateMassifs),
			})
			s.NoError(err)
			firstMassifFilename := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, uint32(tt.firstUpdateMassifs)))
			firstHash := mustHashFile(s.T(), firstMassifFilename)

			// Add the content for the second update

			if tt.secondUpdateMassifs > 0 {
				massifLeaves := mmr.HeightIndexLeafCount(uint64(tt.massifHeight - 1)) // = ((2 << massifHeight) - 1 + 1) >> 1
				// CreateLog always deleted blobs, so we can only use AddLeavesToLog here
				for range tt.secondUpdateMassifs {
					tc.AddLeavesToLog(
						tenantId0, tt.massifHeight, int(massifLeaves),
						massifs.TestWithSealKey(&key),
					)
				}
			}

			if tt.secondUpdateExtraLeaves > 0 {
				tc.AddLeavesToLog(
					tenantId0, tt.massifHeight, int(tt.secondUpdateExtraLeaves),
					massifs.TestWithSealKey(&key),
				)
			}

			// Replicate the content
			err = app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", tt.massifHeight),
				"replicate-logs",
				"--replicadir", replicaDir,
				"--massif", fmt.Sprintf("%d", tt.firstUpdateMassifs+tt.secondUpdateMassifs),
			})
			s.NoError(err)
			// note: secondMassifFilename *may* be same as first depending on test config
			secondMassifFilename := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, uint32(tt.firstUpdateMassifs+tt.secondUpdateMassifs)))
			secondHash := mustHashFile(s.T(), secondMassifFilename)

			assert.NotEqual(s.T(), firstHash, secondHash, "the massif should have changed")

			// Attempt to replicate again, this will verify the local state and then do nothing
			err = app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", tt.massifHeight),
				"replicate-logs",
				"--replicadir", replicaDir,
				"--massif", fmt.Sprintf("%d", tt.firstUpdateMassifs+tt.secondUpdateMassifs),
			})

			s.NoError(err)
		})
	}

}

// TestV0ToV1ReplicationTransition tests that v0 seal replica can be continued with v1 seals
// In this tests the log starts with v0 seals, is replicated, and the continues with v1 seals.
// This covers the production case where there are previously replicated logs.
func (s *ReplicateLogsCmdSuite) TestV0ToV1ReplicationTransition() {

	logger.New("TestV0ToV1ReplicationTransition")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestV0ToV1ReplicationTransition")

	h8MassifLeaves := mmr.HeightIndexLeafCount(uint64(8 - 1)) // = ((2 << massifHeight) - 1 + 1) >> 1

	tests := []struct {
		name                 string
		massifHeight         uint8
		legacyCount          uint64
		lastLeagacyLeafCount uint64
		// if zero, the last legacy massif will be completed. If the last legacy is full and v1Count is zero the test is invalid
		v1Count         uint64
		lastV1LeafCount uint64
	}{
		// make sure we cover the obvious edge cases
		{name: "complete first massif with v0 promoted to v1", massifHeight: 8, legacyCount: 0, lastLeagacyLeafCount: h8MassifLeaves - 3, v1Count: 0, lastV1LeafCount: 3},
	}
	key := massifs.TestGenerateECKey(s.T(), elliptic.P256())

	for _, tt := range tests {

		s.Run(tt.name, func() {

			// Populate the log with content under legacy seals

			require.True(s.T(), tt.legacyCount > 0 || tt.lastLeagacyLeafCount > 0, uint32(0), "invalid test")
			require.True(s.T(), tt.v1Count > 0 || tt.lastV1LeafCount > 0, uint32(0), "invalid test")
			replicaDir := s.T().TempDir()
			tenantId0 := tc.G.NewTenantIdentity()
			// leagacyLeafCount = massifLeaves*tt.legacyCount + tt.lastLeagacyLeafCount

			// note: CreateLog both creates the massifs *and* populates them
			lastMassif := uint32(tt.legacyCount)
			if lastMassif > 0 {
				lastMassif--
			}

			// If we skip CreateLog below, we need to delete the blobs
			tc.AzuriteContext.DeleteBlobsByPrefix(massifs.TenantMassifPrefix(tenantId0))

			if lastMassif > 0 {
				tc.CreateLog(
					tenantId0, tt.massifHeight, lastMassif,
					massifs.TestWithSealKey(&key), massifs.TestWithV0Seals(),
				)
			}
			if tt.lastLeagacyLeafCount > 0 {
				tc.AddLeavesToLog(
					tenantId0, tt.massifHeight, int(tt.lastLeagacyLeafCount),
					massifs.TestWithSealKey(&key), massifs.TestWithV0Seals(),
				)
			}

			// Replicate the log
			// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
			app := veracity.NewApp("tests", true)
			veracity.AddCommands(app, true)

			err := app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", tt.massifHeight),
				"replicate-logs",
				// "--ancestors", fmt.Sprintf("%d", tt.ancestors),
				"--replicadir", replicaDir,
				"--massif", fmt.Sprintf("%d", lastMassif),
			})
			s.NoError(err)

			// Add v1 sealed content
			lastMassif = uint32(tt.v1Count)
			if lastMassif > 0 {
				lastMassif--
			}

			// Add v1 sealed content
			if lastMassif > 0 {
				massifLeaves := mmr.HeightIndexLeafCount(uint64(tt.massifHeight - 1)) // = ((2 << massifHeight) - 1 + 1) >> 1
				// CreateLog always deleted blobs, so we can only use AddLeavesToLog here
				for range tt.v1Count {
					tc.AddLeavesToLog(
						tenantId0, tt.massifHeight, int(massifLeaves),
						massifs.TestWithSealKey(&key), /*, massifs.TestWithV0Seals() V1 seals*/
					)
				}
			}
			if tt.lastLeagacyLeafCount > 0 {
				tc.AddLeavesToLog(
					tenantId0, tt.massifHeight, int(tt.lastV1LeafCount),
					massifs.TestWithSealKey(&key), /*, massifs.TestWithV0Seals() V1 seals*/
				)
			}

			// Replicate the v1 content
			err = app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", tt.massifHeight),
				"replicate-logs",
				"--replicadir", replicaDir,
				"--massif", fmt.Sprintf("%d", lastMassif),
			})
			s.NoError(err)

			// re-read the v1 seal and decide we are up to date
			err = app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", tt.massifHeight),
				"replicate-logs",
				"--replicadir", replicaDir,
				"--massif", fmt.Sprintf("%d", lastMassif),
			})

			s.NoError(err)
		})
	}

}

// TestReplicatingMassifLogsForOneTenant test that by default af full replica is made
func (s *ReplicateLogsCmdSuite) TestReplicatingMassifLogsForOneTenant() {

	logger.New("Test4AzuriteMassifsForOneTenant")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "Test4AzuriteMassifsForOneTenant")

	massifHeight := uint8(8)

	tests := []struct {
		massifCount uint32
	}{
		// make sure we cover the obvious edge cases
		{massifCount: 1},
		{massifCount: 2},
		{massifCount: 5},
	}

	for _, tt := range tests {

		massifCount := tt.massifCount

		s.Run(fmt.Sprintf("massifCount:%d", massifCount), func() {
			tenantId0 := tc.G.NewTenantIdentity()

			// note: CreateLog both creates the massifs *and* populates them
			tc.CreateLog(tenantId0, massifHeight, massifCount)

			replicaDir := s.T().TempDir()

			// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
			app := veracity.NewApp("tests", true)
			veracity.AddCommands(app, true)

			err := app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", massifHeight),
				"replicate-logs",
				"--replicadir", replicaDir,
				"--massif", fmt.Sprintf("%d", massifCount-1),
			})
			s.NoError(err)

			for i := range massifCount {
				expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
				s.FileExistsf(expectMassifFile, "the replicated massif should exist")
				expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
				s.FileExistsf(expectSealFile, "the replicated seal should exist")
			}
		})
	}
}

// TestAncestorMassifsForOneTenant tests that the --ancestors option
// limits the number of historical massifs that are replicated Note that
// --ancestors=0 still requires consistency against local replica of the remote
func (s *ReplicateLogsCmdSuite) TestAncestorMassifLogsForOneTenant() {

	logger.New("Test4AzuriteMassifsForOneTenant")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "Test4AzuriteMassifsForOneTenant")

	massifHeight := uint8(8)

	tests := []struct {
		massifCount uint32
		ancestors   uint32
	}{
		// make sure we cover the obvious edge cases
		{massifCount: 1, ancestors: 1},
		{massifCount: 2, ancestors: 1},
		{massifCount: 5, ancestors: 1},
		{massifCount: 1, ancestors: 2},
		{massifCount: 2, ancestors: 2},
		{massifCount: 5, ancestors: 2},
		{massifCount: 2, ancestors: 3},
		{massifCount: 5, ancestors: 3},

		{massifCount: 5},
		{massifCount: 1},
		{massifCount: 2},
	}

	for _, tt := range tests {

		massifCount := tt.massifCount

		s.Run(fmt.Sprintf("massifCount:%d", massifCount), func() {
			tenantId0 := tc.G.NewTenantIdentity()

			// note: CreateLog both creates the massifs *and* populates them
			tc.CreateLog(tenantId0, massifHeight, massifCount)

			replicaDir := s.T().TempDir()

			// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
			app := veracity.NewApp("tests", true)
			veracity.AddCommands(app, true)

			err := app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", massifHeight),
				"replicate-logs",
				"--ancestors", fmt.Sprintf("%d", tt.ancestors),
				"--replicadir", replicaDir,
				"--massif", fmt.Sprintf("%d", massifCount-1),
			})
			s.NoError(err)

			if tt.ancestors >= massifCount-1 {
				// then all massifs should be replicated
				for i := range massifCount {
					expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
					s.FileExistsf(expectMassifFile, "the replicated massif should exist")
					expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
					s.FileExistsf(expectSealFile, "the replicated seal should exist")
				}
				return
			}

			// To allow testing cases where the ancestors are greater than the count, we need to guard against underflow here.
			end := max(2, massifCount) - 2 - tt.ancestors

			for i := range end {
				expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
				s.NoFileExistsf(expectMassifFile, "the replicated massif should NOT exist")
				expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
				s.NoFileExistsf(expectSealFile, "the replicated seal should NOT exist")
			}

			for i := massifCount - 1 - tt.ancestors; i < massifCount; i++ {
				expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
				s.FileExistsf(expectMassifFile, "the replicated massif should exist")
				expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
				s.FileExistsf(expectSealFile, "the replicated seal should exist")
			}
		})
	}
}

// TestSparseReplicaCreatedAfterExtendedOffline tests that the --ancestors
// option limits the number of historical massifs that are replicated, and in
// the case where the verifier has been off line for a long, the resulting
// replica is sparse. --ancestors is set what the user wants to have a bound on
// the work done in any one run
func (s *ReplicateLogsCmdSuite) TestSparseReplicaCreatedAfterExtendedOffline() {

	logger.New("TestSparseReplicaCreatedAfterExtendedOffline")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestSparseReplicaCreatedAfterExtendedOffline")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	// This test requires two invocations. For the first invocation, we make ony one massif available.
	// Then after that is successfully replicated, we add the rest of the massifs.

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, 1)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err := app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		//  --ancestors defaults to 0 which means "all", but only massif is available
		"--replicadir", replicaDir,
		"--massif", "0",
	})
	s.NoError(err)

	// add the rest of the massifs
	leavesPerMassif := mmr.HeightIndexLeafCount(uint64(massifHeight - 1))
	for i := uint32(1); i < massifCount; i++ {
		tc.AddLeavesToLog(tenantId0, massifHeight, int(leavesPerMassif))
	}

	// This call, due to the --ancestors=1, should only replicate the last
	// massif, and this will leave a gap in the local replica. Imporantly, this
	// means the remote log has not been checked as being consistent with the
	// local. The supported way to fill the gaps is to run with --ancestors=0 (which is the default)
	// this will fill the gaps and ensure remote/local consistency
	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		"--ancestors", "1", // this will replicate massif 2 & 3
		"--replicadir", replicaDir,
		"--massif", fmt.Sprintf("%d", massifCount-1),
	})
	s.NoError(err)

	// check the 0'th massifs and seals was replicated (by the first run of veractity)
	expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, 0))
	s.FileExistsf(expectMassifFile, "the replicated massif should exist")
	expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, 0))
	s.FileExistsf(expectSealFile, "the replicated seal should exist")

	// check the gap was not mistakenly filled
	for i := uint32(1); i < massifCount-2; i++ {
		expectMassifFile = filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
		s.NoFileExistsf(expectMassifFile, "the replicated massif should NOT exist")
		expectSealFile = filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
		s.NoFileExistsf(expectSealFile, "the replicated seal should NOT exist")
	}

	// check the massifs from the second veracity run were replicated
	for i := massifCount - 2; i < massifCount; i++ {
		expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
		s.FileExistsf(expectMassifFile, "the replicated massif should exist")
		expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
		s.FileExistsf(expectSealFile, "the replicated seal should exist")
	}
}

// TestFullReplicaByDefault tests that we get a full replica when
// updating a previous replica after further massifs have been added
func (s *ReplicateLogsCmdSuite) TestFullReplicaByDefault() {

	logger.New("TestFullReplicaByDefault")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestFullReplicaByDefault")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	// This test requires two invocations. For the first invocation, we make ony one massif available.
	// Then after that is successfully replicated, we add the rest of the massifs.

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, 1)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err := app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		//  --ancestors defaults to 0 which means "all", but only massif is available
		"--replicadir", replicaDir,
		"--massif", "0",
	})
	s.NoError(err)

	// add the rest of the massifs
	leavesPerMassif := mmr.HeightIndexLeafCount(uint64(massifHeight - 1))
	for i := uint32(1); i < massifCount; i++ {
		tc.AddLeavesToLog(tenantId0, massifHeight, int(leavesPerMassif))
	}

	// This call, due to the --ancestors=0 default, should replicate all the new massifs.
	// The previously replicated massifs should not be re-verified.
	// The first new replicaetd massif should be verified as consistent with the
	// last local massif. This last point isn't assured by this test, but if
	// debugging it, that behviour can be observed.
	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		//  --ancestors defaults to 0 which means "all", but only massif is available
		"--replicadir", replicaDir,
		"--massif", fmt.Sprintf("%d", massifCount-1),
	})
	s.NoError(err)

	// check the 0'th massifs and seals was replicated (by the first run of veractity)
	expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, 0))
	s.FileExistsf(expectMassifFile, "the replicated massif should exist")
	expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, 0))
	s.FileExistsf(expectSealFile, "the replicated seal should exist")

	// check the massifs from the second veracity run were replicated
	for i := uint32(1); i < massifCount; i++ {
		expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
		s.FileExistsf(expectMassifFile, "the replicated massif should exist")
		expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
		s.FileExistsf(expectSealFile, "the replicated seal should exist")
	}
}

// TestLocalTamperDetected tests that a localy tampered masif is detected
//
// In this case, an attacker changes a remotely replicated massif in an attempt to
// include, exclude or change some element. In order for such a change to be
// provable, the attacker has to re-build the log from the point of the tamper
// forward, otherwise the inclusion proof for the changed element will fail.  We
// can simulate this situation without re-building the log simply by changing
// one of the peaks, as a re-build will necessarily always result in a different
// peak value.
//
// Attacks where the leaves are changed or remove and the log is not re-built
// can only be deteceted by full audit anyway. But these attacks are essentially
// equivalent to data corruption. And they do not result in a log which includes
// a different thing, just a single entry (or pair of) in the log which can't be
// proven
func (s *ReplicateLogsCmdSuite) TestLocalTamperDetected() {

	logger.New("TestFullReplicaByDefault")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestFullReplicaByDefault")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	// This test requires two invocations. For the first invocation, we make ony
	// one massif available.  Then after that is successfully replicated, we
	// tamper a peak in the local replica, then attempt to replicate the
	// subsequent log - this should fail due to the local data being unable to
	// re-produce the root needed for the local seal to verify.

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, 1)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err := app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		//  --ancestors defaults to 0 which means "all", but only massif is available
		"--replicadir", replicaDir,
		"--massif", "0",
	})
	s.NoError(err)

	localReader := newTestLocalReader(s.T(), replicaDir, massifHeight)

	massifLeafCount := mmr.HeightIndexLeafCount(uint64(massifHeight) - 1)
	LastLeafIndex := massifLeafCount - 1
	mmrSize0 := mmr.FirstMMRSize(mmr.MMRIndex(LastLeafIndex))
	peaks := mmr.Peaks(mmrSize0 - 1)
	// this simulates the effect of changing a leaf then re-building the log so
	// that a proof of inclusion can be produced for the new element, this
	// necessarily causes a peak to change. *any* peak change will cause the
	// consistency proof to fail. And regardless of whether our seals are
	// accumulators (all peak hashes) or a single bagged peak, the local log
	// will be unable to produce the correct detached payload for the Sign1 seal
	// over the root material.
	tamperLocalReaderNode(s.T(), localReader, tenantId0,
		massifHeight, peaks[len(peaks)-1], []byte{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F})

	// Note: it's actually a property of the way massifs fill that the last node
	// added is always a peak, we could have taken that shortcut abvove. In the
	// interests of illustrating how any peak can be found, its done the long
	// way above.

	// add the rest of the massifs
	for i := uint32(1); i < massifCount; i++ {
		tc.AddLeavesToLog(tenantId0, massifHeight, int(massifLeafCount))
	}

	// This call, due to the --ancestors=0 default, should replicate all the new massifs.
	// The previously replicated massifs should not be re-verified.
	// The first new replicaetd massif should be verified as consistent with the
	// last local massif. This last point isn't assured by this test, but if
	// debugging it, that behviour can be observed.
	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		//  --ancestors defaults to 0 which means "all", but only massif is available
		"--replicadir", replicaDir,
		"--massif", fmt.Sprintf("%d", massifCount-1),
	})

	s.ErrorIs(err, massifs.ErrSealVerifyFailed)

	// check the 0'th massifs and seals was replicated (by the first run of veractity)
	expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, 0))
	s.FileExistsf(expectMassifFile, "the replicated massif should exist")
	expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, 0))
	s.FileExistsf(expectSealFile, "the replicated seal should exist")

	// check the massifs from the second veracity run were NOT replicated
	for i := uint32(1); i < massifCount; i++ {
		expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
		s.NoFileExistsf(expectMassifFile, "the replicated massif should NOT exist")
		expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
		s.NoFileExistsf(expectSealFile, "the replicated seal should NOT exist")
	}
}

// Test4MassifsForThreeTenants multiple massifs are replicated
// when the output of the watch command is provided on stdin
func (s *ReplicateLogsCmdSuite) Test4MassifsForThreeTenants() {

	logger.New("Test4AzuriteMassifsForThreeTenants")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "Test4AzuriteMassifsForThreeTenants")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, massifCount)
	tenantId1 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId1, massifHeight, massifCount)
	tenantId2 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId2, massifHeight, massifCount)

	changes := []struct {
		TenantIdentity string `json:"tenant"`
		MassifIndex    int    `json:"massifindex"`
	}{
		{tenantId0, int(massifCount - 1)},
		{tenantId1, int(massifCount - 1)},
		{tenantId2, int(massifCount - 1)},
	}

	data, err := json.Marshal(changes)
	s.NoError(err)
	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(data)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		"--replicadir", replicaDir,
	})
	s.NoError(err)

	for _, change := range changes {
		for i := range change.MassifIndex + 1 {
			expectMassifFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeMassifPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(
				expectMassifFile, "the replicated massif should exist")
			expectSealFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeSealPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(expectSealFile, "the replicated seal should exist")
		}
	}
}

// TestThreeTenantsOneAtATime uses --concurency to force the replication to go one tenant at a time
// The test just ensures the obvious boundary case works
func (s *ReplicateLogsCmdSuite) TestThreeTenantsOneAtATime() {
	logger.New("TestThreeTenantsOneAtATime")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestThreeTenantsOneAtATime")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, massifCount)
	tenantId1 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId1, massifHeight, massifCount)
	tenantId2 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId2, massifHeight, massifCount)

	changes := []struct {
		TenantIdentity string `json:"tenant"`
		MassifIndex    int    `json:"massifindex"`
	}{
		{tenantId0, int(massifCount - 1)},
		{tenantId1, int(massifCount - 1)},
		{tenantId2, int(massifCount - 1)},
	}

	data, err := json.Marshal(changes)
	s.NoError(err)
	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(data)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		"--replicadir", replicaDir,
		"--concurrency", "1",
	})
	s.NoError(err)

	for _, change := range changes {
		for i := range change.MassifIndex + 1 {
			expectMassifFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeMassifPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(
				expectMassifFile, "the replicated massif should exist")
			expectSealFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeSealPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(expectSealFile, "the replicated seal should exist")
		}
	}
}

// TestConcurrencyZero uses --concurency to force the replication to go one tenant at a time
// The test just ensures the obvious boundary case works
func (s *ReplicateLogsCmdSuite) TestConcurrencyZero() {
	logger.New("TestConcurrencyZero")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestConcurrencyZero")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, massifCount)
	tenantId1 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId1, massifHeight, massifCount)
	tenantId2 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId2, massifHeight, massifCount)

	changes := []struct {
		TenantIdentity string `json:"tenant"`
		MassifIndex    int    `json:"massifindex"`
	}{
		{tenantId0, int(massifCount - 1)},
		{tenantId1, int(massifCount - 1)},
		{tenantId2, int(massifCount - 1)},
	}

	data, err := json.Marshal(changes)
	s.NoError(err)
	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(data)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		"--replicadir", replicaDir,
		"--concurrency", "0",
	})
	s.NoError(err)

	for _, change := range changes {
		for i := range change.MassifIndex + 1 {
			expectMassifFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeMassifPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(
				expectMassifFile, "the replicated massif should exist")
			expectSealFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeSealPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(expectSealFile, "the replicated seal should exist")
		}
	}
}

// TestConcurrencyCappedToTenantCount sets --concurency greater than the number of tenants
// and shows all tenants are replicated
func (s *ReplicateLogsCmdSuite) TestConcurrencyCappedToTenantCount() {
	logger.New("TestConcurrencyCappedToTenantCount")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestConcurrencyCappedToTenantCount")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, massifCount)
	tenantId1 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId1, massifHeight, massifCount)
	tenantId2 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId2, massifHeight, massifCount)

	changes := []struct {
		TenantIdentity string `json:"tenant"`
		MassifIndex    int    `json:"massifindex"`
	}{
		{tenantId0, int(massifCount - 1)},
		{tenantId1, int(massifCount - 1)},
		{tenantId2, int(massifCount - 1)},
	}

	data, err := json.Marshal(changes)
	s.NoError(err)
	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(data)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		"--replicadir", replicaDir,
		"--concurrency", "30000",
	})
	s.NoError(err)

	for _, change := range changes {
		for i := range change.MassifIndex + 1 {
			expectMassifFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeMassifPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(
				expectMassifFile, "the replicated massif should exist")
			expectSealFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeSealPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(expectSealFile, "the replicated seal should exist")
		}
	}

}

// newTestLocalReader creates a new LocalReader
// This provides a convenient way to interact with the massifs locally replicated by integration tests.
func newTestLocalReader(
	t *testing.T, replicaDir string, massifHeight uint8) *massifs.LocalReader {
	cache, err := massifs.NewLogDirCache(logger.Sugar, veracity.NewFileOpener())
	require.NoError(t, err)
	localReader, err := massifs.NewLocalReader(logger.Sugar, cache)
	require.NoError(t, err)

	cborCodec, err := massifs.NewRootSignerCodec()
	require.NoError(t, err)

	opts := []massifs.DirCacheOption{
		massifs.WithDirCacheReplicaDir(replicaDir),
		massifs.WithDirCacheMassifLister(veracity.NewDirLister()),
		massifs.WithDirCacheSealLister(veracity.NewDirLister()),
		massifs.WithReaderOption(massifs.WithMassifHeight(massifHeight)),
		massifs.WithReaderOption(massifs.WithSealGetter(&localReader)),
		massifs.WithReaderOption(massifs.WithCBORCodec(cborCodec)),
	}
	cache.ReplaceOptions(opts...)
	return &localReader
}

// Test4MassifsForThreeTenantsFromFile multiple massifs are replicated
// when the output of the watch command is provided in a file on disc
func (s *ReplicateLogsCmdSuite) Test4MassifsForThreeTenantsFromFile() {

	logger.New("Test4AzuriteMassifsForThreeTenantsFromFile")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "Test4AzuriteMassifsForThreeTenantsFromFile")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	// note: CreateLog both creates the massifs *and* populates them
	tc.CreateLog(tenantId0, massifHeight, massifCount)
	tenantId1 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId1, massifHeight, massifCount)
	tenantId2 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId2, massifHeight, massifCount)

	changes := []struct {
		TenantIdentity string `json:"tenant"`
		MassifIndex    int    `json:"massifindex"`
	}{
		{tenantId0, int(massifCount - 1)},
		{tenantId1, int(massifCount - 1)},
		{tenantId2, int(massifCount - 1)},
	}

	data, err := json.Marshal(changes)
	s.NoError(err)
	// note: the suite does a before & after pipe for Stdin

	replicaDir := s.T().TempDir()

	inputFilename := filepath.Join(s.T().TempDir(), "input.json")
	createFileFromData(s.T(), data, inputFilename)

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--height", fmt.Sprintf("%d", massifHeight),
		"replicate-logs",
		"--replicadir", replicaDir,
		"--changes", inputFilename,
	})
	s.NoError(err)

	for _, change := range changes {
		for i := range change.MassifIndex + 1 {
			expectMassifFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeMassifPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(
				expectMassifFile, "the replicated massif should exist")
			expectSealFile := filepath.Join(
				replicaDir, massifs.ReplicaRelativeSealPath(change.TenantIdentity, uint32(i)))
			s.FileExistsf(expectSealFile, "the replicated seal should exist")
		}
	}
}

func createFileFromData(t *testing.T, data []byte, filename string) {
	f, err := os.Create(filename)
	require.NoError(t, err)
	defer f.Close()
	n, err := f.Write(data)
	require.NoError(t, err)
	require.Equal(t, n, len(data))
}

// tamperLocalReaderNode over-writes the log entry at the given mmrIndex with the provided bytes
// This is typically used to simulate a local tamper or coruption
//
// The value needs to be non-empty and no longer than LogEntryBytes, a fine
// value for this purpose is:
//
//	[]byte{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F}
func tamperLocalReaderNode(
	t *testing.T, reader *massifs.LocalReader, tenantIdentity string,
	massifHeight uint8, mmrIndex uint64, tamperedValue []byte) {

	require.NotZero(t, len(tamperedValue))
	require.LessOrEqual(t, len(tamperedValue), massifs.LogEntryBytes)

	leafIndex := mmr.LeafIndex(mmrIndex)
	massifIndex := massifs.MassifIndexFromLeafIndex(massifHeight, leafIndex)
	mc, err := reader.GetMassif(context.TODO(), tenantIdentity, massifIndex)
	require.NoError(t, err)

	i := mmrIndex - mc.Start.FirstIndex
	logData := mc.Data[mc.LogStart():]
	copy(logData[i*massifs.LogEntryBytes:i*massifs.LogEntryBytes+8], tamperedValue)

	filePath := reader.GetMassifLocalPath(tenantIdentity, uint32(massifIndex))
	f, err := os.Create(filePath) // read-write & over write
	require.NoError(t, err)
	defer f.Close()
	n, err := f.Write(mc.Data)
	require.NoError(t, err)
	require.Equal(t, n, len(mc.Data))
}
