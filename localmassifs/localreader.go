package localmassifs

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

type ReadOpener struct{}

func (*ReadOpener) Open(name string) (io.ReadCloser, error) {
	fpath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return os.Open(fpath)
}

func NewFileOpener() massifs.Opener {
	return &ReadOpener{}
}

type StdinOpener struct {
	data []byte
}

func NewStdinOpener() massifs.Opener {
	return &StdinOpener{}
}

func (o *StdinOpener) Open(string) (io.ReadCloser, error) {
	if len(o.data) > 0 {
		return io.NopCloser(bytes.NewReader(o.data)), nil
	}

	r := bufio.NewReader(os.Stdin)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	o.data = data
	return io.NopCloser(bytes.NewReader(o.data)), nil
}

// Utilities to remove the os dependencies from the MassifReader
type OsDirLister struct{}
type SuffixDirLister struct {
	OsDirLister
	Suffix string
}

func NewDirLister() massifs.DirLister {
	return &OsDirLister{}
}
func NewSuffixDirLister(suffix string) massifs.DirLister {
	return &SuffixDirLister{Suffix: suffix}
}
func (*OsDirLister) ListFiles(name string) ([]string, error) {
	dpath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	result := []string{}
	entries, err := os.ReadDir(dpath)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		// if !entry.IsDir() && entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), massifs.V1MMRMassifExt){
		if !entry.IsDir() {
			result = append(result, filepath.Join(dpath, entry.Name()))
		}
	}
	return result, nil
}

func (s *SuffixDirLister) ListFiles(name string) ([]string, error) {
	found, err := s.OsDirLister.ListFiles(name)
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, f := range found {
		if strings.HasSuffix(f, s.Suffix) {
			matched = append(matched, f)
		}
	}
	return matched, nil
}
