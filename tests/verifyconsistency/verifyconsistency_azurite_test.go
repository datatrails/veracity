//go:build integration && azurite

package verifyconsistency

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/veracity"
)

func (s *VerifyConsistencyCmdSuite) Test4AzuriteMassifsForOneTenant() {

	logger.New("TestLocalMassifReaderGetVerifiedContext")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestLocalMassifReaderGetVerifiedContext")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId0, massifHeight, uint64(massifCount*(1<<massifHeight)+0))

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
}

func (s *VerifyConsistencyCmdSuite) Test4AzuriteMassifsForThreeTenants() {

	logger.New("TestLocalMassifReaderGetVerifiedContext")
	defer logger.OnExit()

	tc := massifs.NewLocalMassifReaderTestContext(
		s.T(), logger.Sugar, "TestLocalMassifReaderGetVerifiedContext")

	massifCount := uint32(4)
	massifHeight := uint8(8)

	tenantId0 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId0, massifHeight, uint64(massifCount*(1<<massifHeight)+0))
	tenantId1 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId1, massifHeight, uint64(massifCount*(1<<massifHeight)+0))
	tenantId2 := tc.G.NewTenantIdentity()
	tc.CreateLog(tenantId2, massifHeight, uint64(massifCount*(1<<massifHeight)+0))

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
