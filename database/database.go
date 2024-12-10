package database

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type Database struct {
	Lock   sync.Mutex
	Cli    *gorm.DB
	Logger zerolog.Logger
	DryRun bool
}

func (d *Database) GetSource(ctx context.Context, path string) (*BackupSource, error) {
	d.Lock.Lock()
	defer d.Lock.Unlock()

	d.Logger.Debug().Str("path", path).Msg("get source")

	source := &Source{}
	err := d.Cli.Where(Source{Path: path}).FirstOrCreate(source).Error
	if err != nil {
		return nil, err
	}

	return &BackupSource{db: d, record: source, logger: d.Logger.With().Str("source", path).Logger()}, nil
}
