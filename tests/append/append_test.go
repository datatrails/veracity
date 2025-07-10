package append

import (
	"os"

	"github.com/datatrails/veracity"
)

// Test that
func (s *AppendCmdSuite) TestAppendCCFSignedStatement() {
	// replicaDir := s.T().TempDir()

	var err error

	err = os.Chdir("/Users/robin/Desktop/personal/ietf/data")
	s.Require().NoError(err, "should be able to change directory to /Users/robin/Desktop/ietf/data")

	app := veracity.NewApp("tests", true)
	veracity.AddCommands(app, true)

	err = app.Run([]string{
		"veracity",
		"-t", "tenant/6a009b40-eb55-4159-81f0-69024f89f53c",
		// "-l", "v1/mmrs/tenant/6a009b40-eb55-4159-81f0-69024f89f53c/0/massifs/0000000000000000.log",
		"-l", "/Users/robin/Desktop/personal/ietf/data",
		// "append" , "--generate-sealer-key",
		"append", "--sealer-key", "ecdsa-key-private.cbor", "--signed-statement", "in-toto.json.hashenvelope.cose.empty_uhdr",
	})
	s.NoError(err)
}
