package fileutils

import (
	"context"
)

// WatchFile watches a file for changes and emits an event when it changes.
func WatchFile(ctx context.Context, path string, ticker <-chan struct{}, onErr func(err error)) (chan struct{}, error) {
	ch := make(chan struct{})

	lastHash, err := ComputeFileHash(path)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker:
				newHash, err := ComputeFileHash(path)
				if err != nil {
					onErr(err)
				}
				if newHash != 0 &&
					lastHash != newHash {
					lastHash = newHash
					ch <- struct{}{}
				}
			}
		}
	}()

	return ch, nil
}
