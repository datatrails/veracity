package verifyevents

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

// VerifyEventsSuite deals with veracity based verification that DataTrails events are included in a Merkle Log
type VerifyEventsSuite struct {
	tests.IntegrationTestSuite
}

func TestVerifyEventsSuite(t *testing.T) {

	suite.Run(t, new(VerifyEventsSuite))
}
