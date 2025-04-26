package asset

import (
	"time"

	"github.com/rs/zerolog"
)

type Asset interface {
	zerolog.LogObjectMarshaler
	Path() string
	Name() string // base name of the file
	Size() int64  // length in bytes for regular files
	ModTime() time.Time
	// compute hash from the filesystem
	ComputeHash() (uint64, error)
}

type ArchivedAsset interface {
	Asset
	SourcePath() string  // path of the source where the asset was found
	StoredHash() uint64  // hash of the asset stored in the archive
	ArchivePath() string // path of the archive containing the file
}
