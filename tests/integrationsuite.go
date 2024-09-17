package tests

import (
	"bytes"
	"io"
	"os"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	// These constants are well known and described here:
	// See: https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite

	AzureStorageAccountVar    string = "AZURE_STORAGE_ACCOUNT"
	AzureStorageKeyVar        string = "AZURE_STORAGE_KEY"
	AzuriteBlobEndpointURLVar string = "AZURITE_BLOB_ENDPOINT_URL"

	AzuriteWellKnownKey             string = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	AzuriteWellKnownAccount         string = "devstoreaccount1"
	AzuriteWellKnownBlobEndpointURL string = "http://127.0.0.1:10000/devstoreaccount1/"
	AzuriteResourceGroup            string = "azurite-emulator"
	AzuriteSubscription             string = "azurite-emulator"
)

/**
 * Suites can be used to bundle tests,
 *  add fixtures for reliable test setup and teardown,
 *  as well as collect and handle test stats within the suite.
 */

// IntegrationTestSuite base for all tests
// It's an integration test because we don't require a specifically deployed
// instance, we rely on the availability of prod.
type IntegrationTestSuite struct {
	suite.Suite

	Env          TestEnv
	origStdin    *os.File
	stdinWriter  *os.File
	stdinReader  *os.File
	origStdout   *os.File
	stdoutWriter *os.File
	stdoutReader *os.File
}

// StdinWriteAndClose writes the provided bytes to std in and closes the write
// side of pipe It should be called to provided input for any test that read
// stdin. os.Stdin is set to the read side of the pipe in BeforeTest, and
// restored in AfterTest.
func (s *IntegrationTestSuite) StdinWriteAndClose(b []byte) (int, error) {
	require := s.Require()
	require.NotNil(s.stdinWriter)
	n, err := s.stdinWriter.Write(b)

	// close regardless of error
	s.stdinWriter.Close()
	s.stdinWriter = nil
	return n, err
}

func (s *IntegrationTestSuite) SetupSuite() {
	// capture this as early as possible
	s.origStdin = os.Stdin
	s.origStdout = os.Stdout
}

// EnsureAzuriteEnv ensures the environment variables for azurite are set
// But respects any that are already set
func (s *IntegrationTestSuite) EnsureAzuriteEnv() {

	for _, varval := range []struct {
		key   string
		value string
	}{
		{"VERACITY_IKWID", "1"},
		{AzureStorageAccountVar, AzuriteWellKnownAccount},
		{AzureStorageKeyVar, AzuriteWellKnownKey},
		{AzuriteBlobEndpointURLVar, AzuriteWellKnownBlobEndpointURL},
	} {
		if os.Getenv(varval.key) == "" {
			err := os.Setenv(varval.key, varval.value)
			require.NoError(s.T(), err)
		}
	}
}

func (s *IntegrationTestSuite) ReplaceStdin() {
	var err error
	require := s.Require()
	require.NotNil(s.origStdin)
	s.restoreStdin()
	s.stdinReader, s.stdinWriter, err = os.Pipe()
	require.NoError(err)
	os.Stdin = s.stdinReader
}

func (s *IntegrationTestSuite) ReplaceStdout() {
	var err error
	require := s.Require()
	require.NotNil(s.origStdout)
	s.restoreStdout()
	s.stdoutReader, s.stdoutWriter, err = os.Pipe()
	require.NoError(err)
	os.Stdout = s.stdoutWriter
}

func (s *IntegrationTestSuite) CaptureAndCloseStdout() string {
	s.Require().NotNil(s.stdoutReader)
	s.stdoutWriter.Close()
	s.stdoutWriter = nil
	var buf bytes.Buffer
	_, err := io.Copy(&buf, s.stdoutReader)
	s.Require().NoError(err)
	s.restoreStdout()
	return buf.String()
}

func (s *IntegrationTestSuite) restoreStdin() {
	os.Stdin = s.origStdin

	if s.stdinWriter != nil {
		s.stdinWriter.Close()
	}
	if s.stdinReader != nil {
		s.stdinReader.Close()
	}
	s.stdinWriter = nil
	s.stdinReader = nil
}

func (s *IntegrationTestSuite) restoreStdout() {
	os.Stdout = s.origStdout

	if s.stdoutWriter != nil {
		s.stdoutWriter.Close()
	}
	if s.stdoutReader != nil {
		s.stdoutReader.Close()
	}
	s.stdoutWriter = nil
	s.stdoutReader = nil
}

// BeforeTest is run before the test
//
// It gets the correct suite wide test environment
// As well as makes a test specific test tenant
func (s *IntegrationTestSuite) BeforeTest(suiteName, testName string) {

	var err error
	require := s.Require()
	s.ReplaceStdin()

	// Note: do NOT replace stdout by default

	logger.New("NOOP")
	defer logger.OnExit()

	// get values we need from the environment
	s.Env, err = NewTestEnv()
	require.NoError(err)
}

// AfterTest is run after the test has executed
//
// Currently used to print useful information for failing tests
func (s *IntegrationTestSuite) AfterTest(suiteName, testName string) {

	s.restoreStdin()
}
