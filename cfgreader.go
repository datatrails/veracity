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
func cfgReader(cmd *CmdCtx, cCtx *cli.Context) error {
	var err error
	var reader azblob.Reader

	if cmd.log == nil {
		if err = cfgLogging(cmd, cCtx); err != nil {
			return err
		}
	}

	container := cCtx.String("container")

	account := cCtx.String("account")
	url := cCtx.String("url")

	if account == "" {
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
			return err
		}
		cmd.reader = reader
		return nil
	}

	if url == "" {
		url = fmt.Sprintf(AzureBlobURLFmt, account)
	}
	if !strings.HasSuffix(url, "/") {
		url = url + "/"
	}

	reader, err = azblob.NewReaderNoAuth(account, url, container)
	if err != nil {
		return fmt.Errorf("failed to connect to blob store: %v", err)
	}
	cmd.reader = reader

	return nil
}
