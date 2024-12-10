package asset

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/rs/zerolog"
)

const fourGiB = 2 << 31

func NewFromFS(path string, info fs.FileInfo) (Asset, error) {
	mode := info.Mode()
	if !mode.IsRegular() {
		return nil, errors.New("not a regular file")
	}

	if info.Size() > fourGiB {
		return nil, fmt.Errorf("%w: current size %d, maximum %d", ErrMaxSizeExceeded, info.Size(), fourGiB)
	}

	asset := &fsAsset{
		path: path,
		info: info,
	}

	return asset, nil
}

type fsAsset struct {
	path string
	info fs.FileInfo
}

// Name implements Asset.
func (a *fsAsset) Name() string {
	return a.info.Name()
}

// Size implements Asset.
func (a *fsAsset) Size() int64 {
	return a.info.Size()
}

// ModTime implements Asset.
func (a *fsAsset) ModTime() time.Time {
	return a.info.ModTime()
}

// MarshalZerologObject implements Asset.
func (a *fsAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", a.path)
	e.Str("name", a.info.Name())
	e.Int64("size", a.info.Size())
}

// Path implements Asset.
func (a *fsAsset) Path() string {
	return a.path
}
