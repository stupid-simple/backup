package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/database"
	"github.com/stupid-simple/backup/ziparchiver"
)

func restoreCommand(ctx context.Context, args RestoreCommand, logger zerolog.Logger) error {
	if args.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	if args.Database == "" {
		return fmt.Errorf("must specify database")
	}

	destPath := args.Dest

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

	dbCli, err := newSQLite(args.Database, logger)
	if err != nil {
		return err
	}

	db := &database.Database{
		Cli:    dbCli,
		Logger: logger,
		DryRun: args.DryRun,
	}

	restoreDest, err := db.GetSource(ctx, args.Dest)
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
		ziparchiver.WithRestoreDryRun(args.DryRun),
	)
}
