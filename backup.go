package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/i-segura/snapsync/backup"
	"github.com/i-segura/snapsync/database"
	"github.com/rs/zerolog"
)

func backupCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Backup.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	srcPath := args.Backup.Source

	startTime := time.Now()
	logger.Info().Str("source", srcPath).Msg("starting backup")
	defer func() {
		tookSeconds := time.Since(startTime).Seconds()
		if ctx.Err() != nil {
			logger.Info().Str("source", srcPath).Float64("seconds", tookSeconds).Msg("backup cancelled")
		} else {
			logger.Info().Str("source", srcPath).Float64("seconds", tookSeconds).Msg("backup done")
		}
	}()

	db, err := newSQLite(args.Backup.Database, logger, args.Backup.DryRun)
	if err != nil {
		return err
	}

	err = backup.BackupSource(ctx, backup.BackupParams{
		SourcePath: srcPath,
		DestPath:   args.Backup.Dest,
		DB: &database.Database{
			Cli:    db,
			Logger: logger,
			DryRun: args.Backup.DryRun,
		},
		Logger: logger,
	},
		backup.WithDryRun(args.Backup.DryRun),
		backup.WithArchivePrefix(args.Backup.ArchivePrefix),
	)
	if err != nil {
		return err
	}

	return nil
}

func randomDirectoryArchiveFunc(destPath string) func() string {
	return func() string {
		return filepath.Join(destPath, fmt.Sprintf("%d.zip", time.Now().UTC().UnixMilli()))
	}
}
