package main

import (
	"context"
	"fmt"
	"iter"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/database"
)

func cleanCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Clean.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	startTime := time.Now()
	logger.Info().Msg("starting cleaning old backup files")
	defer func() {
		tookSeconds := time.Since(startTime).Seconds()
		if ctx.Err() != nil {
			logger.Info().Float64("seconds", tookSeconds).Msg("cleaning cancelled")
		} else {
			logger.Info().Float64("seconds", tookSeconds).Msg("cleaning done")
		}
	}()

	dbCli, err := newSQLite(args.Clean.Database, logger)
	if err != nil {
		return err
	}

	db := &database.Database{
		Cli:    dbCli,
		Logger: logger,
		DryRun: args.Clean.DryRun,
	}

	return cleanOldBackupFiles(ctx, cleanParams{
		sourcePath:    args.Clean.Source,
		limitArchives: args.Clean.ArchiveLimit,
		dryRun:        args.Clean.DryRun,
		db:            db,
		logger:        logger,
	})
}

type cleanParams struct {
	sourcePath    string
	limitArchives int
	dryRun        bool
	db            *database.Database
	logger        zerolog.Logger
}

func cleanOldBackupFiles(ctx context.Context, p cleanParams) error {
	var sources iter.Seq[*database.BackupSource]
	if p.sourcePath == "" {
		var err error
		sources, err = p.db.IterSources(ctx)
		if err != nil {
			return err
		}
	} else {
		src, err := p.db.GetSource(ctx, p.sourcePath)
		if err != nil {
			return err
		}
		sources = func(yield func(*database.BackupSource) bool) {
			yield(src)
		}
	}

	totalSizeFreed := int64(0)
	filesDeleted := 0
	for src := range sources {
		logger := p.logger.With().Str("source", src.Path()).Logger()
		if ctx.Err() != nil {
			break
		}

		findOpts := []database.FindArchivesOptions{
			database.WithFindArchivesOrderBy(database.FindArchivesOrderBySize),
			database.WithFindArchivesOnlyFullyBackedUp(),
		}
		if p.limitArchives > 0 {
			findOpts = append(findOpts, database.WithFindArchivesLimit(p.limitArchives))
		}

		archivePaths := []string{}
		{
			seq, err := src.FindArchives(ctx, findOpts...)
			if err != nil {
				logger.Error().Err(err).Msg("failed to find archives")
				continue
			}
			for archive := range seq {
				if ctx.Err() != nil {
					break
				}
				logger.Info().
					Str("path", archive.Path).
					Int64("files_size", archive.Size).
					Int("files_count", archive.AssetCount).
					Msg("found old archive")
				archivePaths = append(archivePaths, archive.Path)
			}
		}
		if len(archivePaths) == 0 {
			logger.Info().Msg("no old archives found")
			continue
		}

		err := src.DeleteArchives(ctx, archivePaths)
		if err != nil {
			return fmt.Errorf("error deleting old backup data from registry: %w", err)
		}

		logger.Info().Interface("files", archivePaths).Msg("deleting old backup files")

		for _, path := range archivePaths {
			stat, err := os.Stat(path)
			if err != nil {
				logger.Error().Err(err).Str("path", path).Msg("failed to stat old backup file")
				continue
			}

			if err := os.Remove(path); err != nil {
				logger.Error().Err(err).Str("path", path).
					Int64("size", stat.Size()).
					Msg("failed to delete old backup file")
			} else {
				logger.Info().Str("path", path).Int64("size", stat.Size()).Msg("deleted old backup file")
				totalSizeFreed += stat.Size()
				filesDeleted++
			}
		}
	}

	if totalSizeFreed > 0 {
		p.logger.Info().
			Int("files_deleted", filesDeleted).
			Int64("total_freed", totalSizeFreed).
			Msg("deleted old backup files")
	}

	return nil
}
