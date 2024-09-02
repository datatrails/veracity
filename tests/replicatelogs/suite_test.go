package verifyconsistency

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

type ReplicateLogsCmdSuite struct {
	tests.IntegrationTestSuite
}

func (s *ReplicateLogsCmdSuite) SetupSuite() {
	s.IntegrationTestSuite.SetupSuite()
	// ensure we have the azurite config in the env for all the tests so that --envauth always uses the emulator
	s.EnsureAzuriteEnv()
}

func TestReplicateLogsCmdSuite(t *testing.T) {

	suite.Run(t, new(ReplicateLogsCmdSuite))
}
