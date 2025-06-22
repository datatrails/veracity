package findtrieentries

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

// FindTrieEntriesSuite deals with tests around finding trie entries
type FindTrieEntriesSuite struct {
	tests.IntegrationTestSuite
}

func TestFindTrieEntriesSuite(t *testing.T) {

	suite.Run(t, new(FindTrieEntriesSuite))
}
