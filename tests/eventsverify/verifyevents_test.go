//go:build integration && prodpublic

package verifyevents

import (
	"github.com/datatrails/veracity"
	"github.com/datatrails/veracity/tests/katdata"
)

// VerifyOneEventStdIn tests that the veracity sub command verify-included
// can verify a single event in the format returned from a direct get.
// The event is provided on standard input
func (s *VerifyEventsSuite) TestVerifyOneEventStdIn() {
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
		"verify-included",
	})
	assert.NoErrorf(err, "the event is a known good event from the public production tenant, yet verification has failed")
}

func (s *VerifyEventsSuite) TestOneTamperEventStdIn() {

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
		"verify-included",
	})
	assert.Errorf(err, "the event should not have verified, its data was purposefully tampered")
}
