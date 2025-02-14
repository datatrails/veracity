package findmmrentries

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

// FindMMREntriesSuite deals with tests around finding mmr entries
type FindMMREntriesSuite struct {
	tests.IntegrationTestSuite
}

func TestFindMMREntriesSuite(t *testing.T) {

	suite.Run(t, new(FindMMREntriesSuite))
}
