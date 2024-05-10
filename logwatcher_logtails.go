package veracity

import (
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/datatrails/forestrie/go-forestrie/massifs"
)

// LogTail records the newest (highest numbered) massif path in a log It is used
// to represent both the most recent massif log blob, and the most recent massif
// seal blob
type LogTail struct {
	Tenant string
	Path   string
	Number int
	Ext    string
}

// NewLogTail parses the log tail information from path and returns a LogTail
func NewLogTail(path string) (LogTail, error) {
	number, ext, err := ParseMassifPathNumberExt(path)
	if err != nil {
		return LogTail{}, err
	}
	tenant, err := ParseMassifPathTenant(path)
	if err != nil {
		return LogTail{}, err
	}

	return LogTail{
		Tenant: tenant,
		Path:   path,
		Number: number,
		Ext:    ext,
	}, nil
}

// TryReplacePath considers if the other path is more recent. If it is, it
// replaces the values on the current tail with those parsed from other and
// returns true.  Returns false if other is older than the tail.
func (l *LogTail) TryReplacePath(path string) bool {

	if l.Ext == massifs.V1MMRMassifExt && !IsMassifPathLike(path) {
		return false
	}

	if l.Ext == massifs.V1MMRSealSignedRootExt && !IsSealPathLike(path) {
		return false
	}

	number, ext, err := ParseMassifPathNumberExt(path)
	if err != nil {
		return false
	}
	if number <= l.Number {
		return false
	}
	l.Number = number
	l.Ext = ext
	return true
}

// TryReplaceTail considers if the other tail is more recent If it is, it
// replaces the values on the current tail with those copied from other and
// returns true.  Returns false if other is older than the tail.
func (l *LogTail) TryReplaceTail(other LogTail) bool {

	// The replacement needs to be for the other log
	if l.Tenant != other.Tenant || l.Ext != other.Ext {
		return false
	}

	if other.Number <= l.Number {
		return false
	}
	l.Path = other.Path
	l.Number = other.Number
	l.Ext = other.Ext

	return true
}

// LogTailCollator is used to collate the most recently modified massif blob paths for all tenants in a given time horizon
type LogTailCollator struct {
	massifs map[string]LogTail
	seals   map[string]LogTail
}

// NewLogTailCollator creates a log tail collator
func NewLogTailCollator() LogTailCollator {
	return LogTailCollator{
		massifs: make(map[string]LogTail),
		seals:   make(map[string]LogTail),
	}
}

// sortMapOfLogTails returns a sorted list of the keys to map of LogTails
func sortMapOfLogTails(m map[string]LogTail) []string {
	// The go lang community seems pretty divided on O(1)iterators, and I think this is still "the way"
	keys := make([]string, 0, len(m))
	for k, _ := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// SortedMassifTenants returns the keys of the massifs map in sorted order
func (c LogTailCollator) SortedMassifTenants() []string {
	return sortMapOfLogTails(c.massifs)
}

// SortedSealedTenants returns the keys of the massifs map in sorted order
func (c LogTailCollator) SortedSealedTenants() []string {
	return sortMapOfLogTails(c.seals)
}

// collectPageItem is typically used to handle the first item in a page prior to processing the remainder in a loop
func (c *LogTailCollator) collectPageItem(it *azblob.FilterBlobItem) error {
	lt, err := NewLogTail(*it.Name)
	if err != nil {
		return err
	}

	if lt.Ext == massifs.V1MMRMassifExt {
		cur, ok := c.massifs[lt.Tenant]
		if !ok {
			c.massifs[lt.Tenant] = lt
			return nil
		}
		if cur.TryReplaceTail(lt) {
			c.massifs[lt.Tenant] = cur
		}
		return nil
	}

	// We only support 2 extensions, if we reach here we have excluded ".log" so
	// we know we have a seal
	cur, ok := c.seals[lt.Tenant]
	if !ok {
		c.seals[lt.Tenant] = lt
		return nil
	}
	if cur.TryReplaceTail(lt) {
		c.seals[lt.Tenant] = cur
	}
	return nil
}

// CollatePage process a single page of azure blob filter results and collates
// the most recent LogTail's for each tenant represented in the page.
func (c *LogTailCollator) CollatePage(page []*azblob.FilterBlobItem) error {
	if len(page) == 0 {
		return nil
	}

	for _, it := range page {
		err := c.collectPageItem(it)
		if err != nil {
			return err
		}
	}
	return nil
}
