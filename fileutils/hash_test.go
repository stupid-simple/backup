package fileutils_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stupid-simple/backup/fileutils"
)

var data = []byte("hello world")

func TestComputeHash(t *testing.T) {
	r := strings.NewReader(string(data))

	hash, err := fileutils.ComputeHash(r)
	if err != nil {
		t.Fatal(err)
	}

	if hash != 0x45ab6734b21e6968 {
		t.Errorf("expected hash 0x45ab6734b21e6968, got %x", hash)
	}
}

func TestComputeFileHash(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "hello.txt")
	err := os.WriteFile(testPath, data, 0600)
	if err != nil {
		t.Fatal(err)
	}

	hash, err := fileutils.ComputeFileHash(testPath)
	if err != nil {
		t.Fatal(err)
	}

	if hash != 0x45ab6734b21e6968 {
		t.Errorf("expected hash 0x45ab6734b21e6968, got %x", hash)
	}
}
