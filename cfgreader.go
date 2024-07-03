package veracity

import (
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/urfave/cli/v2"
)

const (
	AzureBlobURLFmt       = "https://%s.blob.core.windows.net"
	AzuriteStorageAccount = "devstoreaccount1"
	DefaultContainer      = "merklelogs"
)

// cfgReader establishes the blob read only data accessor
// only azure blob storage is supported. Both emulated and produciton.
func cfgReader(cmd *CmdCtx, cCtx *cli.Context) (azblob.Reader, error) {
	var err error
	var reader azblob.Reader

	if cmd.log == nil {
		if err = cfgLogging(cmd, cCtx); err != nil {
			return nil, err
		}
	}

	container := cCtx.String("container")

	account := cCtx.String("account")
	url := cCtx.String("url")
	envAuth := cCtx.Bool("envauth")

	if account == "" && url == "" {
		account = AzuriteStorageAccount
		cmd.log.Infof("defaulting to the emulator account %s", account)
	}

	if container == "" {
		container = DefaultContainer
		cmd.log.Infof("defaulting to the standard container %s", container)
	}

	if account == AzuriteStorageAccount {
		cmd.log.Infof("using the emulator and authorizing with the well known private key (for production no authorization is required)")
		// reader, err := azblob.NewAzurite(url, container)
		reader, err = azblob.NewDev(azblob.NewDevConfigFromEnv(), container)
		if err != nil {
			return nil, err
		}
		return reader, nil
	}

	if url == "" {
		url = fmt.Sprintf(AzureBlobURLFmt, account)
	}
	if !strings.HasSuffix(url, "/") {
		url = url + "/"
	}

	if envAuth {
		reader, err = azblob.NewDev(azblob.NewDevConfigFromEnv(), container)
		if err != nil {
			return nil, err
		}
		return reader, nil
	}

	reader, err = azblob.NewReaderNoAuth(url, azblob.WithContainer(container), azblob.WithAccountName(account))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to blob store: %v", err)
	}

	return reader, nil
}
