//go:build integration && azurite

package node

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

// NodeSuite deals with veracity based verification that DataTrails events are included in a Merkle Log
type NodeSuite struct {
	tests.IntegrationTestSuite
}

func TestNodeSuite(t *testing.T) {

	suite.Run(t, new(NodeSuite))
}
