package fileutils_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stupid-simple/backup/fileutils"
)

func TestWatchFile_NotChanged(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "hello.txt")
	err := os.WriteFile(testPath, data, 0600)
	if err != nil {
		t.Fatal(err)
	}

	notify := make(chan struct{})
	defer close(notify)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := fileutils.WatchFile(ctx, testPath, notify, func(err error) {
		t.Fatal(err)
	})
	if err != nil {
		t.Fatal(err)
	}

	notify <- struct{}{}

	select {
	case <-watcher:
		t.Errorf("expected no change")
	case <-time.After(1 * time.Second):
		// ok
	}

}

func TestWatchFile_Changed(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "hello.txt")
	err := os.WriteFile(testPath, data, 0600)
	if err != nil {
		t.Fatal(err)
	}

	notify := make(chan struct{})
	defer close(notify)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := fileutils.WatchFile(ctx, testPath, notify, func(err error) {
		t.Fatal(err)
	})
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.Write(data)
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	notify <- struct{}{}

	select {
	case <-watcher:
		// ok
	case <-time.After(1 * time.Second):
		t.Errorf("expected change")
	}
}
