//go:build integration && azurite

package verifyconsistency

import (
	"path/filepath"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/veracity"
)

// Test that the watch command returns no error or that the error is "no changes"
func (s *ReplicateLogsCmdSuite) TestReplicateFirstPublicMassif() {

	replicaDir := s.T().TempDir()

	// NOTE: These will fail in the CI until the prod APIM principal gets the new custom role
	app := veracity.NewApp("tests", false)
	veracity.AddCommands(app, false)

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"--tenant", s.Env.PublicTenantId,
		"replicate-logs",
		"--replicadir", replicaDir,
		"--progress",
		"--massif", "1",
	})
	s.NoError(err)

	expectMassifFile := filepath.Join(replicaDir, massifs.ReplicaRelativeMassifPath(s.Env.PublicTenantId, 0))
	s.FileExistsf(expectMassifFile, "the replicated massif should exist")
	expectSealFile := filepath.Join(replicaDir, massifs.ReplicaRelativeSealPath(s.Env.PublicTenantId, 0))
	s.FileExistsf(expectSealFile, "the replicated seal should exist")
}
