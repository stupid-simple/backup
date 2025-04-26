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
	ComputeHash() (uint64, error)
}

type ArchivedAsset interface {
	Asset
	SourcePath() string // path of the source where the asset was found
	ComputedHash() uint64
	ArchivePath() string // path of the archive containing the file
}
