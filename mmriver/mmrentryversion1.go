// Package mmriver works with the datatrails ledger based on draft-bryce-cose-receipts-mmr-profile
package mmriver

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// MMREntryVersion1 gets the mmr entry for log entry version 1.
// mmr entry format for log entry version 1:
//
// H( domain | mmrSalt | serializedBytes )
//
// where mmrSalt = extraBytes + idtimestamp
//
// NOTE: extraBytes is consistently 24 bytes on the trie value, so we pad/truncate extrabytes here
// to ensure its 24 bytes also. This allows greater consistency and ease of moving between mmrSalt and trieValue
func MMREntryVersion1(extraBytes []byte, idtimestamp uint64, serializedBytes []byte) ([]byte, error) {
	hasher := sha256.New()

	// domain
	hasher.Write([]byte{byte(LeafTypePlain)})

	// mmrSalt

	// ensure extrabytes is 24 bytes long
	extraBytes, err := ConsistentExtraBytesSize(extraBytes)
	if err != nil {
		return nil, err
	}
	hasher.Write(extraBytes)

	// convert idtimestamp to bytes
	idTimestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(idTimestampBytes, idtimestamp)
	hasher.Write(idTimestampBytes)

	// serializedBytes
	hasher.Write(serializedBytes)

	return hasher.Sum(nil), nil
}

func TrimExtraBytes(extraBytes []byte) []byte {
	extraBytesSize := len(extraBytes)

	// larger size need to truncate
	if extraBytesSize > expectedExtraBytesSize {
		extraBytes = extraBytes[:expectedExtraBytesSize]
	}

	// smaller size need to pad
	if extraBytesSize < expectedExtraBytesSize {
		tmp := make([]byte, expectedExtraBytesSize)
		copy(tmp[:extraBytesSize], extraBytes)
		return tmp
	}

	// goldilocks just right
	return extraBytes
}

// consistentExtraBytesSize ensures the given extraBytes is padded/truncated to exactly 24 bytes
func ConsistentExtraBytesSize(extraBytes []byte) ([]byte, error) {
	extraBytesSize := len(extraBytes)

	// larger size need to truncate
	if extraBytesSize > expectedExtraBytesSize {
		return nil, errors.New("extra bytes is too large, maximum extra bytes size is 24")
	}

	// smaller size need to pad
	if extraBytesSize < expectedExtraBytesSize {
		tmp := make([]byte, expectedExtraBytesSize)
		copy(tmp[:extraBytesSize], extraBytes)
		return tmp, nil
	}

	// goldilocks just right
	return extraBytes, nil
}
