package main

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func newSQLite(path string, logger zerolog.Logger) (*gorm.DB, error) {
	cli, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: dbLogger(logger),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		return nil, err
	}

	err = cli.AutoMigrate(&database.Source{}, &database.Archive{}, &database.ArchiveAsset{})
	if err != nil {
		return nil, err
	}

	return cli, nil
}

type dblog struct {
	parent zerolog.Logger
}

// Error implements logger.Interface.
func (d *dblog) Error(_ context.Context, msg string, args ...any) {
	d.parent.Error().Msgf(msg, args...)
}

// Info implements logger.Interface.
func (d *dblog) Info(_ context.Context, msg string, args ...any) {
	d.parent.Info().Msgf(msg, args...)
}

// LogMode implements logger.Interface.
func (d *dblog) LogMode(lvl logger.LogLevel) logger.Interface {
	var zl zerolog.Level
	switch lvl {
	case logger.Info:
		zl = zerolog.InfoLevel
	case logger.Error:
		zl = zerolog.ErrorLevel
	case logger.Warn:
		zl = zerolog.WarnLevel
	case logger.Silent:
		zl = zerolog.Disabled
	default:
		zl = zerolog.Disabled
	}
	return &dblog{parent: d.parent.Level(zl)}
}

// Trace implements logger.Interface.
func (d *dblog) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	e := d.parent.Trace()
	if err != nil {
		e.Err(err)
	}
	e.Time("begin", begin).Func(func(e *zerolog.Event) {
		sql, rows := fc()
		e.Str("sql", sql)
		e.Int64("rows_affected", rows)
	}).Msg("")
}

// Warn implements logger.Interface.
func (d *dblog) Warn(_ context.Context, msg string, args ...any) {
	d.parent.Warn().Msgf(msg, args...)
}

func dbLogger(logger zerolog.Logger) logger.Interface {
	return &dblog{
		parent: logger,
	}
}
