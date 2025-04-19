package ziparchiver

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/asset"
	"github.com/stupid-simple/backup/fileutils"
)

var (
	errSkippedSameFile = errors.New("skipped same file")
	errSkippedModified = errors.New("skipped modified file")
)

func Restore(ctx context.Context, assets iter.Seq[asset.ArchivedAsset], logger zerolog.Logger, opts ...RestoreOption) error {
	o := restoreOptions{}
	for _, applyOpts := range opts {
		applyOpts(&o)
	}

	logger.Info().Msg("start restoring assets")

	var restoredAssets, skippedAssets int
	defer func() {
		if ctx.Err() != nil {
			logger.Info().
				Int("restored", restoredAssets).
				Int("skipped", skippedAssets).
				Msg("cancelled restore")
		} else if restoredAssets == 0 {
			logger.Info().Int("skipped", skippedAssets).Msg("no assets restored")
		} else {
			logger.Info().
				Int("restored", restoredAssets).
				Int("skipped", skippedAssets).Msg("done restoring assets")
		}
	}()

	zipFile := Open()
	defer func() {
		err := zipFile.Close()
		if err != nil {
			logger.Warn().Err(err).Msg("failed to close zip file")
		}
	}()

	throttledLogger := logger.Sample(&zerolog.BurstSampler{
		Burst:  1,
		Period: 1 * time.Second,
	})
	for asset := range assets {
		if ctx.Err() != nil {
			return nil
		}

		f, err := zipFile.Open(asset)
		if err != nil {
			logger.Warn().Err(err).Object("asset", asset).Msg("could not restore asset")
			continue
		}
		defer func() {
			err := f.Close()
			if err != nil {
				logger.Warn().Err(err).Msg("failed to close file")
			}
		}()

		size, err := restoreAsset(f, asset, logger, false, o.dryRun)
		if errors.Is(err, errSkippedSameFile) {
			logger.Debug().Object("asset", asset).Msg("file already present, skipping")
			skippedAssets++
		} else if errors.Is(err, errSkippedModified) {
			logger.Debug().Object("asset", asset).Msg("found existing file. The file has been modified, skipping")
			skippedAssets++
		} else if err != nil {
			logger.Warn().Err(err).Object("asset", asset).Msg("could not restore asset")
		} else {
			logger.Debug().Object("asset", asset).Int64("bytes", size).Msg("restored asset")
			restoredAssets++
		}

		throttledLogger.Info().
			Int("restored", restoredAssets).
			Int("skipped", skippedAssets).
			Msg("restoring assets")
	}

	return nil
}

func restoreAsset(f fs.File, asset asset.ArchivedAsset, logger zerolog.Logger, overwrite bool, dryRun bool) (int64, error) {
	if info, err := os.Stat(asset.Path()); err == nil {
		logger.Debug().Str("path", asset.Path()).Msg("found existing file")

		if info.IsDir() {
			return 0, errors.New("file is a directory")
		}

		if info.ModTime().Compare(asset.ModTime()) == 0 && info.Size() == asset.Size() {
			return 0, errSkippedSameFile
		}

		storedFileHash, err := fileutils.ComputeFileHash(asset.Path())
		if err != nil {
			return 0, err
		}
		if storedFileHash != asset.ComputedHash() && overwrite {
			logger.Info().Str("path", asset.Path()).Msg("found existing file, overwriting")
			if dryRun {
				return 0, nil
			}

			if err := os.Remove(asset.Path()); err != nil {
				return 0, err
			}

			w, err := os.OpenFile(asset.Path(), os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return 0, err
			}
			defer func() {
				err := w.Close()
				if err != nil {
					logger.Warn().Err(err).Msg("failed to close file")
				}
			}()

			return io.Copy(w, f)
		} else if storedFileHash != asset.ComputedHash() {
			return 0, errSkippedModified
		} else {
			// Should be unreachable.
			return 0, errSkippedSameFile
		}
	} else if os.IsNotExist(err) {
		logger.Debug().Str("path", asset.Path()).Msg("file not found, creating")
		if dryRun {
			return 0, nil
		}

		if err := os.MkdirAll(filepath.Dir(asset.Path()), os.ModePerm); err != nil {
			return 0, err
		}

		w, err := os.OpenFile(asset.Path(), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return 0, err
		}
		defer func() {
			err := w.Close()
			if err != nil {
				logger.Warn().Err(err).Msg("failed to close file")
			}
		}()

		return io.Copy(w, f)
	} else {
		return 0, err
	}
}

type zipArchive struct {
	openReaders map[string]*zip.ReadCloser
}

func Open() *zipArchive {
	return &zipArchive{
		openReaders: make(map[string]*zip.ReadCloser),
	}
}

func (z *zipArchive) Close() error {
	for _, reader := range z.openReaders {
		err := reader.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (z *zipArchive) Open(asset asset.ArchivedAsset) (fs.File, error) {
	var err error
	reader, ok := z.openReaders[asset.ArchivePath()]
	if !ok {
		reader, err = zip.OpenReader(asset.ArchivePath())
		if err != nil {
			return nil, err
		}
		z.openReaders[asset.ArchivePath()] = reader
	}

	inArchivePath, err := filepath.Rel(asset.SourcePath(), asset.Path())
	if err != nil {
		return nil, err
	}

	return reader.Open(inArchivePath)
}
