//go:build integration && prodpublic

package watch

import (
	"github.com/datatrails/veracity"
	"github.com/datatrails/veracity/tests/katdata"
)

// Test that the watch command returns no error or that the error is "no changes"
func (s *WatchCmdSuite) TestNoErrorOrNoChanges() {

	// NOTE: These will fail in the CI until the prod APIM principal gets the new custom role
	assert := s.Assert()
	app := veracity.NewApp(true)
	veracity.AddCommands(app, true)

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"watch",
	})

	if err != nil {
		assert.EqualErrorf(err, veracity.ErrNoChanges.Error(), "the only acceptable error is 'no changes'")
	}
}

// Test that when filtering results by an unknown tenant id, the result is no changes
// The watch command does not check wether the tenants to "filter" for actually have logs
func (s *WatchCmdSuite) TestNoChangesForFictitiousTenant() {
	assert := s.Assert()
	app := veracity.NewApp(true)
	veracity.AddCommands(app, true)

	// NOTE: These will fail in the CI until the prod APIM principal gets the new custom role

	err := app.Run([]string{
		"veracity",
		"--data-url", s.Env.VerifiableDataURL,
		"watch",
		"--tenant" s.Env.UknownTenantId
	})
	assert.Equal(err, veracity.ErrNoChanges)
}
