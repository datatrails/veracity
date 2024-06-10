//go:build integration && prodpublic

package ediag

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

// EDiagSuite deals with veracity based verification that DataTrails events are included in a Merkle Log
type EDiagSuite struct {
	tests.IntegrationTestSuite
}

func TestVerifyEventsSuite(t *testing.T) {

	suite.Run(t, new(EDiagSuite))
}
