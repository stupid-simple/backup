package asset

import (
	"context"
	"io/fs"
	"iter"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

func ScanDirectory(ctx context.Context, dirPath string, logger zerolog.Logger) (iter.Seq[Asset], error) {
	return func(yield func(Asset) bool) {
		var scannedCount int
		var statFiles int

		logger = logger.With().Str("dir", dirPath).Logger()
		logger.Info().Msg("start scanning for assets")
		defer func() {
			logger.Info().
				Int("scanned", statFiles).
				Int("scanned_success", scannedCount).
				Str("dir", dirPath).
				Msgf("done scanning assets")
		}()

		throttledLogger := logger.Sample(&zerolog.BurstSampler{
			Burst:  1,
			Period: 1 * time.Second,
		})
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

			if !yield(newAsset) {
				return filepath.SkipAll
			}
			scannedCount++
			logger.Debug().Object("asset", newAsset).Msg("scanned asset")
			throttledLogger.Info().
				Int("scanned", statFiles).
				Int("scanned_success", scannedCount).
				Str("dir", dirPath).Msg("scanning assets")

			return nil
		})
		if err != nil {
			logger.Error().Err(err).Str("path", dirPath).Msg("could not scan path")
		}
	}, nil
}
