//go:build integration

package watch

import (
	"github.com/datatrails/veracity"
)

func (s *WatchCmdSuite) TestNoErrorOrNoChanges() {

	app := veracity.NewApp(false)
	veracity.AddCommands(app, false)

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"watch",
		"--horizon", "100000h", // 11 years, so we are sure we look back far enough to find an event
	})
	s.NoError(err)
}

// Test that when filtering results by an unknown tenant id, the result is no changes
// The watch command does not check wether the tenants to "filter" for actually have logs
func (s *WatchCmdSuite) TestNoChangesForFictitiousTenant() {
	assert := s.Assert()
	app := veracity.NewApp(false)
	veracity.AddCommands(app, false)
	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"--tenant", s.Env.UnknownTenantId,
		"watch",
	})
	assert.Equal(err, veracity.ErrNoChanges)
}

// Test that the watch command returns no error or that the error is "no changes"
func (s *WatchCmdSuite) TestReplicateFirstPublicMassif() {

	// NOTE: These will fail in the CI until the prod APIM principal gets the new custom role
	app := veracity.NewApp(false)
	veracity.AddCommands(app, false)

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"--tenant", s.Env.SynsationTenantId,
		"watch",
		"--horizon", "100000h", // 11 years, so we are sure we look back far enough to find an event
	})
	s.NoError(err)
}
