package zipwriter

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/i-segura/snapsync/fileutils"
)

// Returns zip Writer helper that opens the file upon first write.
func NewLazyZipFile(path string) *ZipFile {
	return &ZipFile{
		lazyOpenFunc: func() (*os.File, error) {
			return openArchiveFile(path)
		},
		delFunc: func() error {
			return os.Remove(path)
		},
	}
}

// Returns zip Writer helper that opens the null device upon first write.
func NewNullZipFile() *ZipFile {
	return &ZipFile{
		lazyOpenFunc: openNullFile,
		delFunc:      func() error { return nil },
	}
}

type ZipFile struct {
	init         bool
	file         *os.File
	writer       *zip.Writer
	lazyOpenFunc func() (*os.File, error)
	delFunc      func() error
}

func (z *ZipFile) Path() string {
	return z.file.Name()
}

// Close the file and writer if it was opened.
func (z *ZipFile) Close() error {
	if !z.init {
		return nil
	}
	defer func() {
		z.init = false
	}()
	err := z.writer.Close()
	return errors.Join(err, z.file.Close())
}

// Delete the file if it was opened.
func (z *ZipFile) Delete() error {
	if !z.init {
		return nil
	}
	return z.delFunc()
}

// CreateHeader creates a new zip entry in the zip file.
func (z *ZipFile) CreateHeader(fh *zip.FileHeader) (io.Writer, error) {
	if !z.init {
		var err error
		z.file, err = z.lazyOpenFunc()
		if err != nil {
			return nil, err
		}
		z.writer = zip.NewWriter(z.file)
		z.init = true
	}

	return z.writer.CreateHeader(fh)
}

func openNullFile() (*os.File, error) {
	return os.OpenFile("/dev/null", os.O_WRONLY, 0600)
}

func openArchiveFile(path string) (*os.File, error) {
	if fileutils.Exists(path) {
		return nil, fmt.Errorf("file or directory already exists with this name: %s", path)
	}

	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
}
