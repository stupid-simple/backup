package main

import (
	"context"
	"iter"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/database"
)

func cleanCommand(ctx context.Context, args CleanCommand, logger zerolog.Logger) error {
	if args.DryRun {
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

	dbCli, err := newSQLite(args.Database, logger)
	if err != nil {
		return err
	}

	db := &database.Database{
		Cli:    dbCli,
		Logger: logger,
		DryRun: args.DryRun,
	}

	return cleanOldBackupFiles(ctx, cleanParams{
		sourcePath:    args.Source,
		limitArchives: args.ArchiveLimit,
		dryRun:        args.DryRun,
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

		seq, err := src.FindArchives(ctx, findOpts...)
		if err != nil {
			logger.Error().Err(err).Msg("failed to find archives")
			continue
		}

		for archive := range seq {
			if ctx.Err() != nil {
				break
			}
			err = src.DeleteArchive(ctx, archive.Path)
			if err != nil {
				logger.Error().Err(err).Str("path", archive.Path).Msg("failed to delete archive record")
				continue
			}

			stat, err := os.Stat(archive.Path)
			if err != nil {
				logger.Error().Err(err).Str("path", archive.Path).Msg("failed to stat old backup file")
				continue
			}

			if err := os.Remove(archive.Path); err != nil {
				logger.Error().Err(err).Str("path", archive.Path).
					Int64("size", stat.Size()).
					Msg("failed to delete old backup file")
			} else {
				logger.Info().Str("path", archive.Path).Int64("size", stat.Size()).Msg("deleted old backup file")
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
