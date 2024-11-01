package veracity

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/datatrails/go-datatrails-common/cbor"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/urfave/cli/v2"
)

func NewReceiptCmd() *cli.Command {
	return &cli.Command{
		Name:  "receipt",
		Usage: "Generate a COSE Receipt of inclusion for any merklelog entry",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
			},
			&cli.Int64Flag{
				Name: "mmrindex", Aliases: []string{"i"},
			},
			&cli.StringFlag{
				Name:    "format",
				Usage:   "override the output `FORMAT`. Supported formats: cbor (binary), hex, base64",
				Aliases: []string{"f"},
				Value:   "hex",
				Action: func(ctx *cli.Context, v string) error {
					if v != "cbor" && v != "base64" && v != "hex" {
						return fmt.Errorf("Unsupported format '%s'. Use one of: cbor, hex, base64", v)
					}
					return nil
				},
			},
		},
		Action: func(cCtx *cli.Context) error {
			cmd := &CmdCtx{}

			var err error

			// This command uses the structured logger for all optional output.
			// Output not explicitly printed is silenced by default.
			if err = cfgLogging(cmd, cCtx); err != nil {
				return err
			}

			log := func(m string, args ...any) {
				cmd.log.Infof(m, args...)
			}

			dataUrl := cCtx.String("data-url")

			reader, err := cfgReader(cmd, cCtx, dataUrl == "")
			if err != nil {
				return err
			}
			tenantIdentity := cCtx.String("tenant")
			if tenantIdentity == "" {
				return fmt.Errorf("tenant identity is required")
			}
			log("verifying for tenant: %s", tenantIdentity)

			mmrIndex := cCtx.Uint64("mmrindex")
			massifHeight := uint8(cCtx.Int64("height"))

			// TODO: local replica receipts, its not a big lift, the local reader used by replicatelogs
			// implements the necessary interface for NewReceipt.
			var cborCodec cbor.CBORCodec
			if cborCodec, err = massifs.NewRootSignerCodec(); err != nil {
				return err
			}
			sealReader := massifs.NewSignedRootReader(cmd.log, reader, cborCodec)
			massifReader := massifs.NewMassifReader(
				cmd.log, reader,
				massifs.WithSealGetter(&sealReader),
				massifs.WithCBORCodec(cborCodec),
			)

			signedReceipt, err := massifs.NewReceipt(
				context.Background(), massifHeight, tenantIdentity, mmrIndex, &massifReader,
			)
			if err != nil {
				return err
			}

			cbor, err := signedReceipt.MarshalCBOR()
			if err != nil {
				return err
			}
			receipt := cbor
			if cCtx.String("format") == "base64" {
				receipt = make([]byte, base64.URLEncoding.EncodedLen(len(cbor)))
				base64.URLEncoding.Encode(receipt, cbor)
			} else if cCtx.String("format") == "hex" {
				receipt = []byte(hex.EncodeToString(cbor))
			}

			if cCtx.String("output") == "" {
				var n int
				n, err = os.Stdout.Write(receipt)
				if err != nil {
					return err
				}
				if n != len(receipt) {
					return fmt.Errorf("failed to write all bytes to stdout")
				}
				return nil
			}

			// Output to file requested
			f, err := os.Create(cCtx.String("output"))
			if err != nil {
				return err
			}
			defer f.Close()
			n, err := f.Write(receipt)
			if err != nil {
				return err
			}
			if n != len(receipt) {
				return fmt.Errorf("failed to write all bytes to file")
			}
			return nil
		},
	}
}
