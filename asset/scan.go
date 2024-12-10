package asset

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/rs/zerolog"
)

func ScanDirectory(ctx context.Context, dirPath string, logger zerolog.Logger) (<-chan Asset, error) {
	scannedCh := make(chan Asset)
	go func() {
		var scannedCount int
		var statFiles int

		logger.Info().Str("path", dirPath).Msg("start scanning for assets")
		defer func() {
			logger.Info().
				Int("scanned", statFiles).
				Int("scanned_success", scannedCount).
				Str("dir", dirPath).
				Msgf("done scanning assets")
		}()
		defer close(scannedCh)
		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return nil
			}

			if err != nil {
				logger.Warn().Err(err).Str("path", path).Msg("could not scan path")
				return nil
			}
			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				logger.Warn().Err(err).Str("path", path).Msg("could not stat path")
				return nil
			}
			mode := info.Mode()
			if !mode.IsRegular() {
				return nil
			}
			statFiles++

			newAsset, err := NewFromFS(path, info)
			if err != nil {
				logger.Warn().Err(err).Str("path", path).Msg("could not create asset")
				return nil
			}

			scannedCh <- newAsset
			scannedCount++
			logger.Debug().Object("asset", newAsset).Msg("scanned asset")

			return nil
		})
		if err != nil {
			logger.Error().Err(err).Str("path", dirPath).Msg("could not scan path")
		}
	}()

	return scannedCh, nil
}
