package asset_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stupid-simple/backup/asset"
)

var data = []byte("hello world")

func TestNewFromFS(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "hello.txt")
	err := os.WriteFile(testPath, data, 0600)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatal(err)
	}

	a, err := asset.NewFromFS(testPath, info)
	if err != nil {
		t.Fatal(err)
	}

	if a.Path() != testPath {
		t.Errorf("expected path %s, got %s", testPath, a.Path())
	}
	if a.Size() != 11 {
		t.Errorf("expected size 11, got %d", a.Size())
	}
	if a.ModTime() != info.ModTime() {
		t.Errorf("expected mod time %s, got %s", info.ModTime(), a.ModTime())
	}
	if a.Name() != "hello.txt" {
		t.Errorf("expected name hello.txt, got %s", a.Name())
	}
	hash, err := a.ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	if hash != 5020219685658847592 {
		t.Errorf("expected hash 5020219685658847592, got %d", hash)
	}
}

func TestNewFromFS_TooLarge(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "hello.txt")
	err := os.WriteFile(testPath, data, 0600)
	if err != nil {
		t.Fatal(err)
	}

	var fourGiB int64 = 4 * 1024 * 1024 * 1024
	a, err := asset.NewFromFS(testPath, fakeFileInfo{name: "hello.txt", size: fourGiB + 1})
	if !errors.Is(err, asset.ErrMaxSizeExceeded) {
		t.Error("expected error")
	}
	if a != nil {
		t.Error("expected nil")
	}
}

type fakeFileInfo struct {
	name string
	size int64
}

// IsDir implements fs.FileInfo.
func (f fakeFileInfo) IsDir() bool {
	return false
}

// ModTime implements fs.FileInfo.
func (f fakeFileInfo) ModTime() time.Time {
	return time.Time{}
}

// Mode implements fs.FileInfo.
func (f fakeFileInfo) Mode() fs.FileMode {
	return 0
}

// Sys implements fs.FileInfo.
func (f fakeFileInfo) Sys() any {
	panic("unimplemented")
}

func (f fakeFileInfo) Name() string {
	return f.name
}

func (f fakeFileInfo) Size() int64 {
	return f.size
}
