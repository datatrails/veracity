package veracity

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/datatrails/forestrie/go-forestrie/massifs"
)

var (
	ErrMassifPathFmt = errors.New("invalid massif path")
)

// XXX: NOTE: Just staging these functions here while the open sourcing effort is in flight

// isMassifPathLike performs a shallow sanity check on a path to see if it could be a massif log path
func IsMassifPathLike(path string) bool {
	if !strings.HasPrefix(path, massifs.V1MMRTenantPrefix) {
		return false
	}
	if !strings.HasSuffix(path, massifs.V1MMRMassifExt) {
		return false
	}
	return true
}

// IsSealPathLike performs a shallow sanity check on a path to see if it could be a massif seal path
func IsSealPathLike(path string) bool {
	if !strings.HasPrefix(path, massifs.V1MMRTenantPrefix) {
		return false
	}
	if !strings.HasSuffix(path, massifs.V1MMRSealSignedRootExt) {
		return false
	}
	return true
}

// ParseMassifPathTenant parse the tenant uuid from a massif storage path
// Performs basic sanity checks
func ParseMassifPathTenant(path string) (string, error) {
	if !strings.HasPrefix(path, massifs.V1MMRTenantPrefix) {
		return "", fmt.Errorf("invalid massif path: %s", path)
	}

	// the +1 strips the leading /
	path = path[len(massifs.V1MMRTenantPrefix)+1:]

	parts := strings.Split(path, massifs.V1MMRPathSep)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid massif path: %s", path)
	}
	// we could parse the uuid, but that seems like over kill
	return parts[0], nil
}

// ParseMassifPathTenant parse the log file number and extension from the storage path
// Performs basic sanity checks
func ParseMassifPathNumberExt(path string) (int, string, error) {
	if !strings.HasPrefix(path, massifs.V1MMRTenantPrefix) {
		return 0, "", fmt.Errorf("%w: %s", ErrMassifPathFmt, path)
	}
	parts := strings.Split(path, massifs.V1MMRPathSep)
	if len(parts) == 0 {
		return 0, "", fmt.Errorf("%w: %s", ErrMassifPathFmt, path)
	}
	base := parts[len(parts)-1]
	parts = strings.Split(base, massifs.V1MMRExtSep)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("%w: base name invalid %s", ErrMassifPathFmt, path)
	}
	if parts[1] != massifs.V1MMRMassifExt && parts[1] != massifs.V1MMRSealSignedRootExt {
		return 0, "", fmt.Errorf("%w: extension invalid %s", ErrMassifPathFmt, path)
	}
	number, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("%w: log file number invalid %s (%v)", ErrMassifPathFmt, path, err)
	}
	return number, parts[1], nil
}
