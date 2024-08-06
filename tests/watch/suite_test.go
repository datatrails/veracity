package verifyevents

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

// VerifyEventsSuite deals with veracity based verification that DataTrails events are included in a Merkle Log
type WatchCmdSuite struct {
	tests.IntegrationTestSuite
}

func TestWatchCmdSuite(t *testing.T) {

	suite.Run(t, new(WatchCmdSuite))
}
