package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/i-segura/snapsync/asset"
	"github.com/i-segura/snapsync/database"
	"github.com/i-segura/snapsync/fileutils"
	"github.com/i-segura/snapsync/ziparchiver"
	"github.com/rs/zerolog"
)

func backupCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Backup.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	if args.Backup.MaxSize.Size > 0 && args.Backup.MaxSize.Size < 1024 {
		return fmt.Errorf("max size must be at least 1024 bytes")
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

	return backupFiles(
		ctx,
		backupParams{
			sourcePath:        srcPath,
			destPath:          args.Backup.Dest,
			archivePrefix:     args.Backup.ArchivePrefix,
			maxFileBytes:      args.Backup.MaxSize.Size,
			includeLargeFiles: args.Backup.IncludeLargeFiles,
			db:                &database.Database{Cli: db, Logger: logger, DryRun: args.Backup.DryRun},
			dryRun:            args.Backup.DryRun,
			logger:            logger,
		},
	)
}

type backupParams struct {
	sourcePath        string
	destPath          string
	archivePrefix     string
	maxFileBytes      int64
	includeLargeFiles bool
	db                *database.Database
	dryRun            bool
	logger            zerolog.Logger
}

func backupFiles(
	ctx context.Context,
	p backupParams,
) error {
	startTime := time.Now()
	p.logger.Info().Str("source", p.sourcePath).Str("dest", p.destPath).Msg("starting backup")
	defer func() {
		tookSeconds := time.Since(startTime).Seconds()
		if ctx.Err() != nil {
			p.logger.Info().Str("source", p.sourcePath).Str("dest", p.destPath).Float64("seconds", tookSeconds).Msg("backup cancelled")
		} else {
			p.logger.Info().Str("source", p.sourcePath).Str("dest", p.destPath).Float64("seconds", tookSeconds).Msg("backup done")
		}
	}()

	destFile, err := os.Open(p.destPath)
	if err != nil {
		return fmt.Errorf("could not open dest path: %w", err)
	}
	info, err := destFile.Stat()
	if err != nil {
		return fmt.Errorf("could not open dest path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("dest path must be a directory and be writable")
	}
	if err = fileutils.VerifyWritable(p.destPath); err != nil {
		return fmt.Errorf("dest path must be writable: %w", err)
	}

	src, err := p.db.GetSource(ctx, p.sourcePath)
	if err != nil {
		return err
	}

	scanned, err := asset.ScanDirectory(ctx, p.sourcePath, p.logger)
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return nil
	}

	return ziparchiver.StoreAssets(
		ctx,
		p.sourcePath,
		ziparchiver.ArchiveDescriptor{
			Dir:    p.destPath,
			Prefix: p.archivePrefix,
		},
		scanned,
		p.logger,
		ziparchiver.WithDryRun(p.dryRun),
		ziparchiver.WithOnlyNewAssets(src),
		ziparchiver.WithRegisterArchivedAssets(src),
		ziparchiver.WithMaxFileBytes(p.maxFileBytes),
		ziparchiver.WithIncludeLargeFiles(p.includeLargeFiles),
	)

}
