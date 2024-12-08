package database

import (
	"time"
)

type Source struct {
	Path      string `gorm:"primaryKey"`
	CreatedAt time.Time
}

type Archive struct {
	Path       string `gorm:"primaryKey"`
	SourcePath string
	Source     Source `gorm:"foreignKey:SourcePath"`
	CreatedAt  time.Time
}

type ArchiveAsset struct {
	ArchivePath string  `gorm:"primaryKey"`
	Path        string  `gorm:"primaryKey"`
	Archive     Archive `gorm:"foreignKey:ArchivePath"`
	Name        string
	Hash        int64
	ModTime     time.Time
	CreatedAt   time.Time
	Size        int64
}
