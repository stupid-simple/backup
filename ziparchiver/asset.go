package ziparchiver

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/asset"
)

type zipAsset struct {
	sourcePath       string
	archivePath      string
	name             string
	path             string
	hash             uint64
	uncompressedSize int64
	modTime          time.Time
}

func (z *zipAsset) SourcePath() string {
	return z.sourcePath
}

func (z *zipAsset) ArchivePath() string {
	return z.archivePath
}

func (z *zipAsset) ComputedHash() uint64 {
	return z.hash
}

// MarshalZerologObject implements asset.Asset.
func (z *zipAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", z.path)
	e.Str("name", z.name)
	e.Uint64("hash", z.hash)
	e.Int64("size", z.uncompressedSize)
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

type readableAsset interface {
	asset.Asset
	Open() (io.ReadCloser, error)
}

type readableFileAsset struct {
	asset.Asset
}

func (r readableFileAsset) Open() (io.ReadCloser, error) {
	return os.Open(r.Path())
}

type readableZipAsset struct {
	asset.ArchivedAsset
	archive *zipArchive
}

func (r readableZipAsset) Open() (io.ReadCloser, error) {
	return r.archive.Open(r.ArchivedAsset)
}
