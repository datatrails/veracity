// Package scitt signed statement conveniences
package scitt

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"

	"github.com/datatrails/veracity/mmriver"
)

const (
	ExtraBytesSize = 24
)

// MMRStatement prepares the details necessary for registering a signed
// statement on a localy forked datatrails ledger replica
type MMRStatement struct {
	CheckedStatement
	// Content is the signed statement raw cbor bytes exactly as read or provide
	Content []byte
	// Hash is th sha256 hash of the Statement
	Hash []byte
	// LeafHash is the MMR ledger defined leaf hash that is added to the ledger
	LeafHash []byte
	// ExtraBytes are the application contribution to the leaf hash. In the case
	// of this pseudo scitt support, it is the first 24 bytes of the Hash
	ExtraBytes []byte
	// The IDTimestamp that contributed to the leaf hash.
	IDTimestamp  uint64
	MMRIndexLeaf uint64
}

type idTimetampGenerator interface {
	NextID() (uint64, error)
}

func NewMMRStatementFromFile(fileName string, idState idTimetampGenerator, policy RegistrationPolicy) (*MMRStatement, *ConciseProblemDetails, error) {
	m := &MMRStatement{}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file %s: %w", fileName, err)
	}

	var cpd *ConciseProblemDetails
	m.CheckedStatement, cpd = RegistrationMandatoryChecks(content, policy)
	if cpd != nil {
		return nil, cpd, fmt.Errorf("failed mandatory registration checks: %s", cpd.Detail)
	}

	m.Content = content
	hasher := sha256.New()
	n, err := hasher.Write(m.Content)
	if err != nil {
		return nil, nil, err
	}
	if n != len(m.Content) {
		return nil, nil, errors.New("hashed to few bytes")
	}
	m.Hash = hasher.Sum(nil)

	// Could use the hash bytes for content addressibility, but its primarily a scitt demo so use subject, but only the first 24 bytes
	// m.ExtraBytes = m.Hash[:ExtraBytesSize]
	m.ExtraBytes = mmriver.TrimExtraBytes([]byte(m.Claims.Subject))

	m.IDTimestamp, err = idState.NextID()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate snowflake id: %w", err)
	}
	// m.IDTimestamp = 0 // XXX: temporary stabilize the hash

	m.LeafHash, err = mmriver.MMREntryVersion1(m.ExtraBytes, m.IDTimestamp, m.Content)
	if err != nil {
		return nil, nil, err
	}
	return m, nil, nil
}
