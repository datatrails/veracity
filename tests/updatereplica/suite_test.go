package updatereplica

import (
	"testing"

	"github.com/datatrails/veracity/tests"
	"github.com/stretchr/testify/suite"
)

type UpdateReplicaCmdSuite struct {
	tests.IntegrationTestSuite
}

func TestUpdateReplicaCmdSuite(t *testing.T) {

	suite.Run(t, new(UpdateReplicaCmdSuite))
}
