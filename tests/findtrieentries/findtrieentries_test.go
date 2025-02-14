package findtrieentries

import (
	"io"
	"os"
	"strings"

	"github.com/datatrails/veracity"
)

const (
	prodPublicTenant   = "tenant/6ea5cd00-c711-3649-6914-7b125928bbb4"
	prodEventsv1Tenant = "tenant/97e90a09-8c56-40df-a4de-42fde462ef6f"
)

// TestAssetsV2EventStdIn tests we can find
// the correct PROD public assetsv2 event trie entry match
func (s *FindTrieEntriesSuite) TestAssetsV2EventStdIn() {
	assert := s.Assert()
	require := s.Require()

	app := veracity.NewApp("version", true)
	veracity.AddCommands(app, true)

	// redirect std out to a known pipe so we can capture it
	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }() // ensure we redirect std out back after

	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	err := app.Run([]string{
		"veracity",
		"find-trie-entries",
		"--log-tenant", prodPublicTenant,
		"--app-id", "assets/20e0864f-423c-4a09-8819-baac2ed326e4/events/effa4ea1-e96d-4272-8c06-c09453c75621", // identity of the event with the public prefix striped
	})
	assert.NoErrorf(err, "the event is a known good event from the public production tenant, yet we have errored trying to find the trie entries")

	writer.Close()
	actualBytes, err := io.ReadAll(reader)
	require.NoError(err)

	// convert the stdout to a string and strip the newlines
	actual := strings.ReplaceAll(string(actualBytes), "\n", "")

	assert.Equal("matches: [27899]", actual)
}

// TestAssetsV2EventAsLeafIndexStdIn tests we can find
// the correct PROD public assetsv2 event trie entry match as
// a leaf index.
func (s *FindTrieEntriesSuite) TestAssetsV2EventAsLeafIndexStdIn() {
	assert := s.Assert()
	require := s.Require()

	app := veracity.NewApp("version", true)
	veracity.AddCommands(app, true)

	// redirect std out to a known pipe so we can capture it
	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }() // ensure we redirect std out back after

	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	err := app.Run([]string{
		"veracity",
		"find-trie-entries",
		"--log-tenant", prodPublicTenant,
		"--app-id", "assets/20e0864f-423c-4a09-8819-baac2ed326e4/events/effa4ea1-e96d-4272-8c06-c09453c75621", // identity of the event with the public prefix striped
		"--massif-start", "1",
		"--massif-end", "1", // event is in massif 1
		"--as-leafindexes",
	})
	assert.NoErrorf(err, "the event is a known good event from the public production tenant, yet we have errored trying to find the trie entries")

	writer.Close()
	actualBytes, err := io.ReadAll(reader)
	require.NoError(err)

	// convert the stdout to a string and strip the newlines
	actual := strings.ReplaceAll(string(actualBytes), "\n", "")

	assert.Equal("matches: [13952]", actual)
}

// TestAssetsV2EventWrongMassifStdIn tests we CANNOT find
// the correct PROD public assetsv2 event trie entry match
// if we set the range of massifs to not include the massif the event is in.
func (s *FindTrieEntriesSuite) TestAssetsV2EventWrongMassifStdIn() {
	assert := s.Assert()
	require := s.Require()

	app := veracity.NewApp("version", true)
	veracity.AddCommands(app, true)

	// redirect std out to a known pipe so we can capture it
	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }() // ensure we redirect std out back after

	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	err := app.Run([]string{
		"veracity",
		"find-trie-entries",
		"--log-tenant", prodPublicTenant,
		"--app-id", "assets/20e0864f-423c-4a09-8819-baac2ed326e4/events/effa4ea1-e96d-4272-8c06-c09453c75621", // identity of the event with the public prefix striped
		"--massif-start", "0",
		"--massif-end", "0", // event is in massif 1
	})
	assert.NoErrorf(err, "the event is a known good event from the public production tenant, yet we have errored trying to find the trie entries")

	writer.Close()
	actualBytes, err := io.ReadAll(reader)
	require.NoError(err)

	// convert the stdout to a string and strip the newlines
	actual := strings.ReplaceAll(string(actualBytes), "\n", "")

	assert.Equal("matches: []", actual)
}

// TestAssetsV2EventCorrectMassifStdIn tests we CAN find
// the correct PROD public assetsv2 event trie entry match
// if we set the range of massifs to include ONLY the massif the event is in.
func (s *FindTrieEntriesSuite) TestAssetsV2EventCorrectMassifStdIn() {
	assert := s.Assert()
	require := s.Require()

	app := veracity.NewApp("version", true)
	veracity.AddCommands(app, true)

	// redirect std out to a known pipe so we can capture it
	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }() // ensure we redirect std out back after

	rescueStderr := os.Stderr
	defer func() { os.Stderr = rescueStderr }() // ensure we redirect std err back after

	readerStdOut, writerStdOut, _ := os.Pipe()
	os.Stdout = writerStdOut

	readerStdErr, writerStdErr, _ := os.Pipe()
	os.Stderr = writerStdErr

	err := app.Run([]string{
		"veracity",
		"--loglevel", "DEBUG",
		"find-trie-entries",
		"--log-tenant", prodPublicTenant,
		"--app-id", "assets/20e0864f-423c-4a09-8819-baac2ed326e4/events/effa4ea1-e96d-4272-8c06-c09453c75621", // identity of the event with the public prefix striped
		"--massif-start", "1",
		"--massif-end", "1", // event is in massif 1
	})
	assert.NoErrorf(err, "the event is a known good event from the public production tenant, yet we have errored trying to find the trie entries")

	writerStdOut.Close()
	actualStdOutBytes, err := io.ReadAll(readerStdOut)
	require.NoError(err)

	writerStdErr.Close()
	actualStdErrBytes, err := io.ReadAll(readerStdErr)
	require.NoError(err)

	// convert the stdout to string and string new lines and convert stderr to string
	actualStdOut := strings.ReplaceAll(string(actualStdOutBytes), "\n", "")
	actualStdErr := string(actualStdErrBytes)

	// assert we are checking the correct massif
	assert.Contains(actualStdErr, "trie entries in massif 1 for matches")

	// assert we are not checking the neighbouring massifs
	assert.NotContains(actualStdErr, "trie entries in massif 0 for matches")
	assert.NotContains(actualStdErr, "trie entries in massif 2 for matches")

	assert.Equal("matches: [27899]", actualStdOut)
}

// TestEventsV1EventStdIn tests we can find
// the correct PROD eventsv1 event trie entry match.
func (s *FindTrieEntriesSuite) TestEventsV1EventStdIn() {
	assert := s.Assert()
	require := s.Require()

	app := veracity.NewApp("version", true)
	veracity.AddCommands(app, true)

	// redirect std out to a known pipe so we can capture it
	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }() // ensure we redirect std out back after

	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	err := app.Run([]string{
		"veracity",
		"find-trie-entries",
		"--log-tenant", prodEventsv1Tenant,
		"--app-id", "events/0194b168-bac0-75e6-bbc4-a47cc45bdbf5", // identity of the event
	})
	assert.NoErrorf(err, "the event is a known good event from the production tenant we are using for test eventsv1 events, yet we have errored trying to find the trie entries")

	writer.Close()
	actualBytes, err := io.ReadAll(reader)
	require.NoError(err)

	// convert the stdout to a string and strip the newlines
	actual := strings.ReplaceAll(string(actualBytes), "\n", "")

	assert.Equal("matches: [4]", actual)
}
