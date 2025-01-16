package app

import (
	"bufio"
	"os"
	"path/filepath"
)

func stdinToAppData() ([]byte, error) {
	return scannerToAppData(bufio.NewScanner(os.Stdin))
}

func filePathToAppData(filePath string) ([]byte, error) {
	filePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	return scannerToAppData(bufio.NewScanner(f))
}

func scannerToAppData(scanner *bufio.Scanner) ([]byte, error) {
	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

// ReadAppData reads the app data from stdin or from a given file path
func ReadAppData(fromStdIn bool, filePath string) ([]byte, error) {

	if fromStdIn {
		return stdinToAppData()
	} else {
		return filePathToAppData(filePath)
	}

}
