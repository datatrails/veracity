package tests

import (
	"fmt"
	"net/url"
	"os"

	"github.com/google/uuid"
)

/**
 * Utility functions for retrieving values for system tests
 *   from the environment.
 */

const (
	// Env keys
	testIDPEnvKey           = "TEST_IDP_KEY"
	fqdnEnvKey              = "FQDN"
	publicTenantIdEnvKey    = "PUBLIC_TENANT_ID"
	verifiableDataURLEnvKey = "VERIFIABLE_DATA_URL"
	publicKeyPrefixEnvKey   = "PUBLIC_KEY"

	// defaults. note because veracity is a customer facing client tool, and
	// because it is specifically concerned with publicly accessible data, we
	// can test it regularly against the production instance.
	productionVerifiableDataUrl = "https://app.datatrails.ai/verifiabledata"
	productionPublicTenantId    = "tenant/6ea5cd00-c711-3649-6914-7b125928bbb4"
)

type TestEnv struct {

	// FQDN of the deployment to test against
	FQDN              string
	VerifiableDataURL string

	// azure blob storage variable
	MerklelogAccountName string
	MerklelogURL         string
	PublicTenantId       string
	MerkelogURLPrefix    string
	PublicKey            string

	UnknownTenantId string
}

// NewTestEnv generates retrieves values from the environment.
func NewTestEnv() (TestEnv, error) {

	publicTenantId := os.Getenv(publicTenantIdEnvKey)
	if publicTenantId == "" {
		publicTenantId = productionPublicTenantId
	}
	verifiableDataURL := os.Getenv(verifiableDataURLEnvKey)
	if verifiableDataURL == "" {
		verifiableDataURL = productionVerifiableDataUrl
	}

	u, err := url.Parse(verifiableDataURL)
	if err != nil {
		return TestEnv{}, err
	}

	env := TestEnv{
		FQDN:              u.Hostname(),
		VerifiableDataURL: verifiableDataURL,
		PublicTenantId:    publicTenantId,
		PublicKey:         os.Getenv(publicKeyPrefixEnvKey),
		UnknownTenantId:   fmt.Sprintf("tenant/%s", uuid.New().String()),
	}

	return env, nil
}
