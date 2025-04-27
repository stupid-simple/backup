package database

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/fileutils"
)

type dbAsset struct {
	record *ArchiveAsset
}

func (d dbAsset) SourcePath() string {
	return d.record.Archive.SourcePath
}

func (d dbAsset) ArchivePath() string {
	return d.record.Archive.Path
}

func (d dbAsset) ComputeHash() (uint64, error) {
	return fileutils.ComputeFileHash(d.record.Path)
}

func (d dbAsset) StoredHash() uint64 {
	return uint64(d.record.Hash)
}

func (d dbAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", d.record.Path)
	e.Str("name", d.record.Name)
	e.Uint64("hash", uint64(d.record.Hash))
	e.Int64("size", d.record.Size)
	e.Str("archive", d.record.Archive.Path)
	e.Str("source", d.record.Archive.SourcePath)
}

func (d dbAsset) ModTime() time.Time {
	return d.record.ModTime
}

func (d dbAsset) Name() string {
	return d.record.Name
}

func (d dbAsset) Path() string {
	return d.record.Path
}

func (d dbAsset) Size() int64 {
	return d.record.Size
}
