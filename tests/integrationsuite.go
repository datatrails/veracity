package tests

import (
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

	Env         TestEnv
	origStdin   *os.File
	stdinWriter *os.File
	stdinReader *os.File
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

// BeforeTest is run before the test
//
// It gets the correct suite wide test environment
// As well as makes a test specific test tenant
func (s *IntegrationTestSuite) BeforeTest(suiteName, testName string) {

	var err error
	require := s.Require()

	s.stdinReader, s.stdinWriter, err = os.Pipe()
	require.NoError(err)
	require.NotNil(s.origStdin)
	os.Stdin = s.stdinReader
	// Note, we don't mess with stdout

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
