package ziparchiver

import (
	"time"

	"github.com/rs/zerolog"
)

type zipAsset struct {
	sourcePath       string
	archivePath      string
	name             string
	path             string
	hash             uint64
	uncompressedSize int64
	compressedSize   int64
	modTime          time.Time
}

func (z *zipAsset) SourcePath() string {
	return z.sourcePath
}

func (z *zipAsset) ArchivePath() string {
	return z.archivePath
}

func (z *zipAsset) ArchivedSize() int64 {
	return z.compressedSize
}

// Hash implements asset.Asset.
func (z *zipAsset) Hash() uint64 {
	return z.hash
}

// MarshalZerologObject implements asset.Asset.
func (z *zipAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", z.path)
	e.Str("name", z.name)
	e.Uint64("hash", z.hash)
	e.Int64("size", z.uncompressedSize)
	e.Int64("compressed_size", z.compressedSize)
	e.Str("archive", z.archivePath)
	e.Str("source", z.sourcePath)
}

// ModTime implements asset.Asset.
func (z *zipAsset) ModTime() time.Time {
	return z.modTime
}

// Name implements asset.Asset.
func (z *zipAsset) Name() string {
	return z.name
}

// Path implements asset.Asset.
func (z *zipAsset) Path() string {
	return z.path
}

// Size implements asset.Asset.
func (z *zipAsset) Size() int64 {
	return z.uncompressedSize
}
