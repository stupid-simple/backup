package main

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/database"
	"github.com/stupid-simple/backup/ziparchiver"
)

func copyCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Copy.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	if args.Copy.MaxSize.Size > 0 && args.Copy.MaxSize.Size < 1024 {
		return fmt.Errorf("max size must be at least 1024 bytes")
	}

	startTime := time.Now()
	logger.Info().Msg("starting compacting")
	defer func() {
		tookSeconds := time.Since(startTime).Seconds()
		if ctx.Err() != nil {
			logger.Info().Float64("seconds", tookSeconds).Msg("compacting cancelled")
		} else {
			logger.Info().Float64("seconds", tookSeconds).Msg("compacting done")
		}
	}()

	dbCli, err := newSQLite(args.Copy.Database, logger)
	if err != nil {
		return err
	}

	db := &database.Database{
		Cli:    dbCli,
		Logger: logger,
		DryRun: args.Copy.DryRun,
	}

	return compactArchives(ctx, compactParams{
		sourcePath:    args.Copy.Source,
		destPath:      args.Copy.Dest,
		archivePrefix: args.Copy.ArchivePrefix,
		maxFileBytes:  args.Copy.MaxSize.Size,
		limitArchives: args.Copy.ArchiveLimit,
		dryRun:        args.Copy.DryRun,
		db:            db,
		logger:        logger,
	})
}

type compactParams struct {
	sourcePath    string
	destPath      string
	archivePrefix string
	maxFileBytes  int64
	limitArchives int
	dryRun        bool
	db            *database.Database
	logger        zerolog.Logger
}

func compactArchives(ctx context.Context, p compactParams) error {

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

	for src := range sources {
		logger := p.logger.With().Str("source", src.Path()).Logger()
		if ctx.Err() != nil {
			break
		}

		findOpts := []database.FindArchivesOptions{
			database.WithFindArchivesOrderBy(database.FindArchivesOrderBySize),
		}
		if p.limitArchives > 0 {
			findOpts = append(findOpts, database.WithFindArchivesLimit(p.limitArchives))
		}
		if p.maxFileBytes > 0 {
			findOpts = append(findOpts, database.WithFindArchivesMaxUncompressedSize(p.maxFileBytes))
		}

		archives := []database.BackupArchive{}
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
					Msg("found archive")
				archives = append(archives, archive)
				archivePaths = append(archivePaths, archive.Path)
			}
		}
		if len(archives) == 0 {
			logger.Info().Msg("no archives found")
			continue
		}

		scanned, err := src.FindArchivedAssets(ctx, database.WithArchiveList(archivePaths))
		if err != nil {
			logger.Error().Err(err).Msg("failed to find archived assets")
			continue
		}

		ziparchiver.CopyArchived(ctx, src.Path(), ziparchiver.ArchiveDescriptor{
			Dir:    p.destPath,
			Prefix: p.archivePrefix,
		},
			scanned,
			logger,
			ziparchiver.WithCopyArchivedDryRun(p.dryRun),
			ziparchiver.WithCopyArchiveRegisterAssets(src),
			ziparchiver.WithCopyArchiveMaxFileBytes(p.maxFileBytes),
		)
	}

	return nil
}
