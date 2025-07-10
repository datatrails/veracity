// Package replicatelogs provides a test suite for the ReplicateLogs command.
package append

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

type AppendCmdSuite struct {
	tests.IntegrationTestSuite
}

func (s *AppendCmdSuite) SetupSuite() {
	s.IntegrationTestSuite.SetupSuite()
	// ensure we have the azurite config in the env for all the tests so that --envauth always uses the emulator
	s.EnsureAzuriteEnv()
}

func TestAppendCmdSuite(t *testing.T) {
	suite.Run(t, new(AppendCmdSuite))
}
