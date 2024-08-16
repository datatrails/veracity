package veracity

import (
	"io"
	"os"
	"path/filepath"
)

type FileWriteOpener struct{}

func (*FileWriteOpener) Open(name string) (io.WriteCloser, error) {
	fpath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return os.Open(fpath)
}

func NewFileWriteOpener() WriteOpener {
	return &FileWriteOpener{}
}
