//go:build integration && prodpublic

package ediag

import (
	"github.com/datatrails/veracity"
	"github.com/datatrails/veracity/tests/katdata"
)

// VerifyOneEventStdIn tests that the veracity sub command events-verify
// can verify a single event in the format returned from a direct get.
// The event is provided on standard input
func (s *EDiagSuite) TestOneEventStdIn() {
	assert := s.Assert()
	app := veracity.NewApp()
	veracity.AddCommands(app)

	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(katdata.KnownGoodPublicEvent)

	err := app.Run([]string{
		"veracity",
		"--loglevel", "INFO",
		"--tenant", s.Env.PublicTenantId,
		"--data-url", s.Env.VerifiableDataURL,
		"ediag",
	})
	assert.NoErrorf(err, "the event is a known good event from the public production tenant, yet verification has failed")
}

func (s *EDiagSuite) TestOneTamperedEventStdIn() {

	assert := s.Assert()
	app := veracity.NewApp()
	veracity.AddCommands(app)

	// note: the suite does a before & after pipe for Stdin
	s.StdinWriteAndClose(katdata.KnownTamperedPublicEvent)

	err := app.Run([]string{
		"veracity",
		"--loglevel", "INFO",
		"--tenant", s.Env.PublicTenantId,
		"--data-url", s.Env.VerifiableDataURL,
		"ediag",
	})
	assert.NoErrorf(err, "while the event is known to be tampered, this command should still succeed")
}
