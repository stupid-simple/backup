package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/database"
	"github.com/stupid-simple/backup/ziparchiver"
)

func compactCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Compact.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	if args.Compact.MaxSize.Size > 0 && args.Compact.MaxSize.Size < 1024 {
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

	dbCli, err := newSQLite(args.Compact.Database, logger, args.Backup.DryRun)
	if err != nil {
		return err
	}

	db := &database.Database{
		Cli:    dbCli,
		Logger: logger,
		DryRun: args.Restore.DryRun,
	}

	sources, err := db.IterSources(ctx)

	for src := range sources {
		if ctx.Err() != nil {
			break
		}

		scanned, err := src.FindArchivedAssets(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("failed to find archived assets")
			continue
		}

		ziparchiver.CopyArchived(ctx, src.Path(), ziparchiver.ArchiveDescriptor{
			Dir:    args.Compact.Dest,
			Prefix: args.Compact.ArchivePrefix,
		},
			scanned,
			logger,
			ziparchiver.WithCopyArchivedDryRun(args.Compact.DryRun),
			ziparchiver.WithCopyArchiveRegisterAssets(src),
			ziparchiver.WithCopyArchiveMaxFileBytes(args.Compact.MaxSize.Size))
	}

	return nil
}
