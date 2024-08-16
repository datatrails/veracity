package veracity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

// support for reading the merkelog signing public key this is currently
// 'distributed' by listing it on the merklelog page of any event in the ui.
// We will need to support key rolling at some point

var (
	ErrInvalidBlockNotPublicKey = errors.New("the data does not have the PEM armour indicating it is a public key")
	// ErrInvalidPublicKeyString     = errors.New("failed to decode the key bytes from a string")
	ErrKeyBytesParseFailed      = errors.New("the pem block could not be parsed as a public key")
	ErrInvalidKeyNotECDSAPublic = errors.New("parsed public key is not the expected ecdsa type")
)

// DecodeECDSAPublicPEM decodes a public pem format ecdsa key
// This is the format that the merklelog signing key is distributed in
func DecodeECDSAPublicPEM(data []byte) (*ecdsa.PublicKey, error) {
	// Decode the PEM block
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, ErrInvalidBlockNotPublicKey
	}

	// Parse the public key
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyBytesParseFailed, err)
	}

	// Type assertion to the expected type, e.g., *ecdsa.PublicKey
	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrInvalidKeyNotECDSAPublic
	}
	return ecdsaPubKey, nil
}

// DecodeECDSAPublicString decodes a public pem format ecdsa key This is the
// format that the merklelog signing key is distributed in, but with the key
// material presented as a single, base64 encoded, string. This is typically
// more convenient for command line and environment vars
func DecodeECDSAPublicString(data string) (*ecdsa.PublicKey, error) {

	keyData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	// Parse the public key
	pubKey, err := x509.ParsePKIXPublicKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyBytesParseFailed, err)
	}

	// Type assertion to the expected type, e.g., *ecdsa.PublicKey
	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrInvalidKeyNotECDSAPublic
	}
	return ecdsaPubKey, nil
}

func CompareECDSAPublicKeys(key1, key2 *ecdsa.PublicKey) bool {
	if key1 == nil || key2 == nil {
		return false
	}

	if !compareCurves(key1.Curve, key2.Curve) {
		return false
	}

	if key1.X.Cmp(key2.X) != 0 || key1.Y.Cmp(key2.Y) != 0 {
		return false
	}

	return true
}

func compareCurves(curve1, curve2 elliptic.Curve) bool {
	if curve1 == nil || curve2 == nil {
		return false
	}

	params1 := curve1.Params()
	params2 := curve2.Params()

	return params1.P.Cmp(params2.P) == 0 &&
		params1.N.Cmp(params2.N) == 0 &&
		params1.B.Cmp(params2.B) == 0 &&
		params1.Gx.Cmp(params2.Gx) == 0 &&
		params1.Gy.Cmp(params2.Gy) == 0 &&
		params1.BitSize == params2.BitSize &&
		params1.Name == params2.Name
}
