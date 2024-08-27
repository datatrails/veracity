package updatereplica

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

type UpdateReplicaCmdSuite struct {
	tests.IntegrationTestSuite
}

func (s *UpdateReplicaCmdSuite) SetupSuite() {
	s.IntegrationTestSuite.SetupSuite()
	// ensure we have the azurite config in the env for all the tests so that --envauth always uses the emulator
	s.EnsureAzuriteEnv()
}

func TestUpdateReplicaCmdSuite(t *testing.T) {

	suite.Run(t, new(UpdateReplicaCmdSuite))
}
