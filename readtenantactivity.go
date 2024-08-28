package veracity

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// TenantMassif identifies a combination of tenant and massif Typically it is
// used to convey that the massif is the most recently changed for that tenant.
// Note: it is a strict subset of the fields in TenantActivity, maintained seperately due to json marshalling
type TenantMassif struct {
	// Massif is the massif index of the most recently appended massif
	Massif int `json:"massifindex"`
	// Tenant is the tenant identity of the most recently changed log
	Tenant string `json:"tenant"`
}

// TenantActivity represents the per tenant output of the watch command
type TenantActivity struct {
	// Massif is the massif index of the most recently appended massif
	Massif int `json:"massifindex"`
	// Tenant is the tenant identity of the most recently changed log
	Tenant string `json:"tenant"`

	// IDCommitted is the idtimestamp for the most recent entry observed in the log
	IDCommitted string `json:"idcommitted"`
	// IDConfirmed is the idtimestamp for the most recent entry to be sealed.
	IDConfirmed  string `json:"idconfirmed"`
	LastModified string `json:"lastmodified"`
	// MassifURL is the remote path to the most recently changed massif
	MassifURL string `json:"massif"`
	// SealURL is the remote path to the most recently changed seal
	SealURL string `json:"seal"`
}

func filePathToTenantMassifs(filePath string) ([]TenantMassif, error) {
	filePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	return scannerToTenantMassifs(bufio.NewScanner(f))
}

func stdinToDecodedTenantMassifs() ([]TenantMassif, error) {
	return scannerToTenantMassifs(bufio.NewScanner(os.Stdin))
}

func scannerToTenantMassifs(scanner *bufio.Scanner) ([]TenantMassif, error) {
	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return TenantMassifsFromData(data)
}

func TenantMassifsFromData(data []byte) ([]TenantMassif, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var doc []TenantMassif
	err := decoder.Decode(&doc)
	if err == nil {
		return doc, nil
	}
	return nil, err
}
