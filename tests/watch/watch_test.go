//go:build integration

package watch

import (
	"github.com/datatrails/veracity"
)

func (s *WatchCmdSuite) TestErrorForNegativeHorizon() {

	app := veracity.NewApp("version", false)
	veracity.AddCommands(app, false)

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"watch",
		"--horizon", "-1h",
	})
	s.ErrorContains(err, "negative horizon")
	// alternative approach which just makes the error more readable
	// s.ErrorContains(err, "is to large or otherwise out of range")
}

func (s *WatchCmdSuite) TestNoErrorVeryLargeHorizon() {

	app := veracity.NewApp("version", false)
	veracity.AddCommands(app, false)

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"watch",
		"--horizon", "1000000000h",
		//"--horizon", "100000h", // 11 years, so we are sure we look back far enough to find an event
	})
	s.NoError(err)
	// alternative approach which just makes the error more readable
	// s.ErrorContains(err, "is out of range or otherwise invalid")
}

func (s *WatchCmdSuite) TestNoErrorLargeButParsableHorizon() {

	app := veracity.NewApp("version", false)
	veracity.AddCommands(app, false)

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"watch",
		"--horizon", "1000000h", // over flows the id timestamp epoch
	})
	s.NoError(err)
	// alternative approach which just makes the error more readable
	// s.ErrorContains(err, "is out of range or otherwise invalid")
}

func (s *WatchCmdSuite) TestNoErrorOrNoChanges() {

	app := veracity.NewApp("version", false)
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
	app := veracity.NewApp("version", false)
	veracity.AddCommands(app, false)
	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"--tenant", s.Env.UnknownTenantId,
		"watch",
	})
	assert.Equal(err, veracity.ErrNoChanges)
}

// Test that the watch command returns no error when the horizon is set longer than the age of the company
func (s *WatchCmdSuite) TestChangesDetected() {

	// NOTE: These will fail in the CI until the prod APIM principal gets the new custom role
	app := veracity.NewApp("version", false)
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
