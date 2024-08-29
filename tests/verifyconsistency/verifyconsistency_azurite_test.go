//go:build integration && azurite

package verifyconsistency

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/veracity"
)

// TestReplicatingMassifLogsForOneTenant test that by default af full replica is made
func (s *VerifyConsistencyCmdSuite) TestReplicatingMassifLogsForOneTenant() {

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
			tc.CreateLog(tenantId0, massifHeight, massifCount)

			replicaDir := s.T().TempDir()

			// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
			app := veracity.NewApp(true)
			veracity.AddCommands(app, true)

			err := app.Run([]string{
				"veracity",
				"--envauth", // uses the emulator
				"--container", tc.TestConfig.Container,
				"--data-url", s.Env.AzuriteVerifiableDataURL,
				"--tenant", tenantId0,
				"--height", fmt.Sprintf("%d", massifHeight),
				"verify-consistency",
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

// TestSingleAncestorMassifsForOneTenant tests that the --ancestors
// option limits the number of historical massifs that are replicated
func (s *VerifyConsistencyCmdSuite) TestSingleAncestorMassifsForOneTenant() {

	logger.New("Test4AzuriteSingleAncestorMassifsForOneTenant")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "Test4AzuriteSingleAncestorMassifsForOneTenant")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId0, massifHeight, massifCount)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp(true)
	veracity.AddCommands(app, true)

	err := app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"verify-consistency",
		"--ancestors", "1",
		"--replicadir", replicaDir,
		"--massif", fmt.Sprintf("%d", massifCount-1),
	})
	s.NoError(err)

	// check the 0'th massifs and seals were _not_ replicated
	expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, 0))
	s.NoFileExistsf(expectMassifFile, "the replicated massif should NOT exist")
	expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, 0))
	s.NoFileExistsf(expectSealFile, "the replicated seal should NOT exist")

	for i := uint32(2); i < massifCount; i++ {
		expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
		s.FileExistsf(expectMassifFile, "the replicated massif should exist")
		expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
		s.FileExistsf(expectSealFile, "the replicated seal should exist")
	}
}

// TestSparseReplicaCreatedAfterExtendedOffline tests that the --ancestors
// option limits the number of historical massifs that are replicated, and in
// the case where the verifier has been off line for a long, the resulting
// replica is sparse. --ancestors is set what the user wants to have a bound on
// the work done in any one run
func (s *VerifyConsistencyCmdSuite) TestSparseReplicaCreatedAfterExtendedOffline() {

	logger.New("Test4AzuriteSingleAncestorMassifsForOneTenant")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "Test4AzuriteSingleAncestorMassifsForOneTenant")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	// This test requires two invocations. For the first invocation, we make ony one massif available.
	// Then after that is successfully replicated, we add the rest of the massifs.

	tenantId0 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId0, massifHeight, 1)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp(true)
	veracity.AddCommands(app, true)

	err := app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"verify-consistency",
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
		"verify-consistency",
		"--ancestors", "1", // this will replicate massif 2 & 3
		"--replicadir", replicaDir,
		"--massif", fmt.Sprintf("%d", massifCount-1),
	})
	s.NoError(err)

	// check the 0'th massifs and seals was replicated (by the first run of veractity)
	expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, 0))
	s.FileExistsf(expectMassifFile, "the replicated massif should NOT exist")
	expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, 0))
	s.FileExistsf(expectSealFile, "the replicated seal should NOT exist")

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
func (s *VerifyConsistencyCmdSuite) TestFullReplicaByDefault() {

	logger.New("TestFullReplicaByDefault")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestFullReplicaByDefault")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	// This test requires two invocations. For the first invocation, we make ony one massif available.
	// Then after that is successfully replicated, we add the rest of the massifs.

	tenantId0 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId0, massifHeight, 1)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp(true)
	veracity.AddCommands(app, true)

	err := app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--tenant", tenantId0,
		"--height", fmt.Sprintf("%d", massifHeight),
		"verify-consistency",
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
		"verify-consistency",
		//  --ancestors defaults to 0 which means "all", but only massif is available
		"--replicadir", replicaDir,
		"--massif", fmt.Sprintf("%d", massifCount-1),
	})
	s.NoError(err)

	// check the 0'th massifs and seals was replicated (by the first run of veractity)
	expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, 0))
	s.FileExistsf(expectMassifFile, "the replicated massif should NOT exist")
	expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, 0))
	s.FileExistsf(expectSealFile, "the replicated seal should NOT exist")

	// check the massifs from the second veracity run were replicated
	for i := uint32(1); i < massifCount; i++ {
		expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(tenantId0, i))
		s.FileExistsf(expectMassifFile, "the replicated massif should exist")
		expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(tenantId0, i))
		s.FileExistsf(expectSealFile, "the replicated seal should exist")
	}
}

// Test4AzuriteMassifsForThreeTenants multiple massifs are replicated
// when the output of the watch command is provided on stdin
func (s *VerifyConsistencyCmdSuite) Test4AzuriteMassifsForThreeTenants() {

	logger.New("Test4AzuriteMassifsForThreeTenants")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "Test4AzuriteMassifsForThreeTenants")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
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
	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(data)

	replicaDir := s.T().TempDir()

	// note: VERACITY_IKWID is set in main, we need it to enable --envauth so we force it here
	app := veracity.NewApp(true)
	veracity.AddCommands(app, true)

	err = app.Run([]string{
		"veracity",
		"--envauth", // uses the emulator
		"--container", tc.TestConfig.Container,
		"--data-url", s.Env.AzuriteVerifiableDataURL,
		"--height", fmt.Sprintf("%d", massifHeight),
		"verify-consistency",
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
