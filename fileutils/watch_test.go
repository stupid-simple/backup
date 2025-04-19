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

	// Create a channel to collect errors from the watcher
	errCh := make(chan error, 1)

	notify := make(chan struct{})
	defer close(notify)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := fileutils.WatchFile(ctx, testPath, notify, func(err error) {
		select {
		case errCh <- err:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make sure file is fully written before continuing.
	time.Sleep(100 * time.Millisecond)

	f, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.Write(data)
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}

	err = f.Sync() // Ensure data is written to disk.
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}

	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure file changes are visible before notifying.
	time.Sleep(100 * time.Millisecond)

	// Signal the watcher to check the file.
	notify <- struct{}{}

	// Check for errors from the watcher.
	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}

	// Wait for file change notification.
	select {
	case <-watcher:
		// ok - change detected.
	case <-time.After(2 * time.Second):
		t.Errorf("expected change but timed out")
	}
}
