package veracity

import (
	"io"
	"os"
	"path/filepath"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

// FileWriteAppendOpener is an interface for opening a file for writing
// The Open implementation must  open for *append*, and must create the file if it does not exist.
// The Create implementation must truncate the file if it exists, and create it if it does not.
type FileWriteAppendOpener struct{}

// Open ensures the named file exists and is writable. Writes are appended to any existing content.
func (*FileWriteAppendOpener) Open(name string) (io.WriteCloser, error) {
	name, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
}

// Create ensures the named file exists, is empty and is writable
// If the named file already exists it is truncated
func (*FileWriteAppendOpener) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

func NewFileWriteOpener() massifs.WriteAppendOpener {
	return &FileWriteAppendOpener{}
}
