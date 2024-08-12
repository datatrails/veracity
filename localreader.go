package veracity

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

type FileOpener struct{}

func (*FileOpener) Open(name string) (io.ReadCloser, error) {
	fpath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return os.Open(fpath)
}

func NewFileOpener() massifs.Opener {
	return &FileOpener{}
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

func NewDirLister() massifs.DirLister {
	return &OsDirLister{}
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
