package fileutils

import (
	"io"
	"os"

	"github.com/cespare/xxhash"
)

// ComputeHash returns the hash of the reader.
// It will read the entire contents of the reader. It will not close the reader.
func ComputeHash(r io.Reader) (uint64, error) {
	hash := xxhash.New()
	_, err := io.Copy(hash, r)
	if err != nil {
		return 0, err
	}
	return hash.Sum64(), nil
}

// ComputeFileHash returns the hash of the file at path.
func ComputeFileHash(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	return ComputeHash(file)
}
