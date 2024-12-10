package main

import (
	"context"
	"fmt"
	"time"

	"github.com/i-segura/snapsync/database"
	"github.com/i-segura/snapsync/ziparchiver"
	"github.com/rs/zerolog"
)

func restoreCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Restore.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	if args.Restore.Database == "" {
		return fmt.Errorf("must specify database")
	}

	destPath := args.Restore.Dest

	startTime := time.Now()
	logger.Info().Str("dest", destPath).Msg("starting restore")
	defer func() {
		tookSeconds := time.Since(startTime).Seconds()
		if ctx.Err() != nil {
			logger.Info().Str("dest", destPath).Float64("seconds", tookSeconds).Msg("restore cancelled")
		} else {
			logger.Info().Str("dest", destPath).Float64("seconds", tookSeconds).Msg("restore done")
		}
	}()

	dbCli, err := newSQLite(args.Restore.Database, logger, args.Restore.DryRun)
	if err != nil {
		return err
	}

	db := &database.Database{
		Cli:    dbCli,
		Logger: logger,
		DryRun: args.Restore.DryRun,
	}

	restoreDest, err := db.GetSource(ctx, args.Restore.Dest)
	if err != nil {
		return err
	}

	assets, err := restoreDest.FindArchivedAssets(ctx)
	if err != nil {
		return err
	}

	return ziparchiver.Restore(
		ctx,
		assets,
		logger.With().Str("dest", destPath).Logger(),
		ziparchiver.WithRestoreDryRun(args.Restore.DryRun),
	)
}
