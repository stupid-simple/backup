package database

import "time"

type BackupArchive struct {
	Path      string
	CreatedAt time.Time
	Size      int64
	AssetCount  int
}
