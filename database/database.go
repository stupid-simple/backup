package database

import (
	"context"
	"iter"
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

func (d *Database) IterSources(ctx context.Context) (iter.Seq[*BackupSource], error) {

	d.Logger.Debug().Msg("get sources")

	sources := []Source{}
	d.Lock.Lock()
	err := d.Cli.WithContext(ctx).Find(&sources).Error
	d.Lock.Unlock()
	if err != nil {
		return nil, err
	}

	return func(yield func(*BackupSource) bool) {
		for _, source := range sources {
			if !yield(&BackupSource{db: d, record: &source, logger: d.Logger.With().Str("source", source.Path).Logger()}) {
				break
			}
		}
	}, nil
}
