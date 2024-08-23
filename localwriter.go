package veracity

import (
	"io"
	"os"
	"path/filepath"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

// FileWriteAppendOpener is an interface for opening a file for writing
// if the file exists, it is opened in APPEND mode
type FileWriteAppendOpener struct{}

func (*FileWriteAppendOpener) Open(name string) (io.WriteCloser, error) {
	name, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
}

func NewFileWriteOpener() massifs.WriteAppendOpener {
	return &FileWriteAppendOpener{}
}
