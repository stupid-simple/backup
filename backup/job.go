package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/i-segura/snapsync/asset"
	"github.com/i-segura/snapsync/database"
	"github.com/i-segura/snapsync/fileutils"
	"github.com/i-segura/snapsync/ziparchiver"
	"github.com/rs/zerolog"
)

type BackupParams struct {
	SourcePath string
	DestPath   string
	DryRun     bool
	DB         *database.Database
	Logger     zerolog.Logger
}

func BackupSource(ctx context.Context, params BackupParams) error {
	logger := params.Logger
	startTime := time.Now()
	logger.Info().Str("source", params.SourcePath).Str("dest", params.DestPath).Msg("starting backup")
	defer func() {
		tookSeconds := time.Since(startTime).Seconds()
		if ctx.Err() != nil {
			logger.Info().Str("source", params.SourcePath).Str("dest", params.DestPath).Float64("seconds", tookSeconds).Msg("backup cancelled")
		} else {
			logger.Info().Str("source", params.SourcePath).Str("dest", params.DestPath).Float64("seconds", tookSeconds).Msg("backup done")
		}
	}()

	destFile, err := os.Open(params.DestPath)
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
	if err = fileutils.VerifyWritable(params.DestPath); err != nil {
		return fmt.Errorf("dest path must be writable: %w", err)
	}

	src, err := params.DB.GetSource(ctx, params.SourcePath)
	if err != nil {
		return err
	}

	scanned, err := asset.ScanDirectory(ctx, params.SourcePath, logger)
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return nil
	}

	return ziparchiver.StoreAssets(
		ctx,
		randomArchiveNameFunc(params.DestPath),
		params.SourcePath,
		scanned,
		logger,
		ziparchiver.WithDryRun(params.DryRun),
		ziparchiver.WithOnlyNewAssets(src),
		ziparchiver.WithRegisterArchivedAssets(src),
	)
}

func randomArchiveNameFunc(dirPath string) func() string {
	return func() string {
		return filepath.Join(dirPath, fmt.Sprintf("%d.zip", time.Now().UTC().UnixMilli()))
	}
}
