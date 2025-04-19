package ziparchiver

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/asset"
	"github.com/stupid-simple/backup/fileutils"
	"github.com/stupid-simple/backup/ziparchiver/zipwriter"
)

type ArchiveDescriptor struct {
	Dir    string // Directory path.
	Prefix string // Can be empty.
}

func StoreAssets(
	ctx context.Context,
	sourcePath string,
	dest ArchiveDescriptor,
	assets iter.Seq[asset.Asset],
	logger zerolog.Logger,
	opts ...StoreOption,
) error {
	o := storeOptions{}
	for _, applyOpts := range opts {
		applyOpts(&o)
	}

	logger = logger.With().Str("source", sourcePath).Str("dest", dest.Dir).Logger()
	logger.Info().Msg("backing up assets")

	var wg sync.WaitGroup
	var storedAssets int
	defer func() {
		wg.Wait()
		if ctx.Err() != nil {
			logger.Info().Int("stored", storedAssets).Msg("cancelled backup")
		} else if storedAssets == 0 {
			logger.Info().Msg("no assets backed up")
		} else {
			logger.Info().Int("stored", storedAssets).Msg("done backing up assets")
		}
	}()

	if o.onlyNewAssets != nil {
		var err error
		assets, err = o.onlyNewAssets.FindMissingAssets(ctx, assets)
		if err != nil {
			return err
		}
	}

	var onArchived func(a asset.ArchivedAsset)
	if o.registerAssets != nil {
		storedCh := make(chan asset.ArchivedAsset)
		defer close(storedCh)
		onArchived = func(a asset.ArchivedAsset) {
			storedCh <- a
			storedAssets++
		}

		wg.Add(1)
		go func() {
			err := o.registerAssets.Register(ctx, iterChannel(ctx, storedCh))
			if err != nil {
				logger.Error().Err(err).Msg("could not register backup assets")
				// Drain the channel.
				for range storedCh {
				}
			}
			wg.Done()
		}()
	} else {
		onArchived = func(asset.ArchivedAsset) {
			storedAssets++
		}
	}

	fullPrefix := filepath.Join(dest.Dir, fmt.Sprintf("%s%d", dest.Prefix, time.Now().UTC().UnixMilli()))

	return writeAssetsToZip(ctx, sourcePath, fullPrefix, assets, onArchived, logger, o)
}

func writeAssetsToZip(
	ctx context.Context,
	sourcePath string,
	fullPrefix string,
	assets iter.Seq[asset.Asset],
	onArchived func(asset.ArchivedAsset),
	logger zerolog.Logger,
	o storeOptions,
) error {
	var zipFile *zipwriter.ZipFile
	if o.dryRun {
		zipFile = zipwriter.NewNullZipFile()
	} else {
		zipFile = newZipFilePart(fullPrefix, 0)
		logger.Info().Str("path", zipFile.Path()).Msg("open archive")
	}
	defer func() {
		if err := zipFile.Close(); err != nil {
			logger.Warn().Err(err).Msg("could not close backup file")
		}
	}()

	var err error
	var written int64
	var part int
	for asset := range assets {
		if ctx.Err() != nil {
			return nil
		}
		if o.maxFileBytes > 0 && asset.Size() >= o.maxFileBytes && !o.includeLargeFiles {
			logger.Warn().
				Object("asset", asset).
				Int64("max_size", o.maxFileBytes).
				Msg("asset larger than max file size. Will be skipped")
			continue
		}
		if o.maxFileBytes > 0 && written+asset.Size() >= o.maxFileBytes {
			logger.Debug().
				Int64("size", asset.Size()).
				Msg("archive size larger than max file size. Will open a new file")
			if err = zipFile.Close(); err != nil {
				logger.Warn().Err(err).Msg("could not close backup file")
			}
			part++
			written = 0
			zipFile = newZipFilePart(fullPrefix, part)
			logger.Info().Str("path", zipFile.Path()).Msg("open archive")
		}

		header := &zip.FileHeader{
			UncompressedSize64: uint64(asset.Size()),
			Modified:           asset.ModTime(),
			Method:             zip.Deflate,
		}
		header.Name, err = filepath.Rel(sourcePath, asset.Path())
		if err != nil {
			logger.Warn().Err(err).Object("asset", asset).Msg("could not backup asset")
			continue
		}

		logger.Debug().Str("relative_path", header.Name).Msg("asset to zip")

		w, err := zipFile.CreateHeader(header)
		if err != nil {
			logger.Warn().Err(err).Object("asset", asset).Msg("could not backup asset")
			continue
		}
		archivedAsset, err := writeAsset(sourcePath, zipFile.Path(), asset, w, logger)
		if err != nil {
			logger.Warn().Err(err).Object("asset", asset).
				Msg("could not backup asset")
		} else {
			logger.Debug().Object("asset", asset).
				Msg("backed up asset")
		}
		written += asset.Size()
		onArchived(archivedAsset)
	}

	return nil
}

func writeAsset(sourcePath string, archivePath string, asset asset.Asset, w io.Writer, logger zerolog.Logger) (asset.ArchivedAsset, error) {
	assetFile, err := os.Open(asset.Path())
	if err != nil {
		return nil, err
	}
	startTime := time.Now()
	defer func() {
		if err := assetFile.Close(); err != nil {
			logger.Warn().Err(err).Msg("failed to close asset file")
		}
		tookSeconds := time.Since(startTime).Seconds()
		logger.Debug().Object("asset", asset).Float64("seconds", tookSeconds).Msg("archived asset")
	}()

	// Write to zip as well as compute hash.
	tee := io.TeeReader(assetFile, w)
	h, err := fileutils.ComputeHash(tee)
	if err != nil {
		return nil, err
	}

	return &zipAsset{
		sourcePath:       sourcePath,
		archivePath:      archivePath,
		name:             asset.Name(),
		path:             asset.Path(),
		hash:             h,
		modTime:          asset.ModTime(),
		uncompressedSize: asset.Size(),
	}, nil
}

func newZipFilePart(fullPrefix string, part int) *zipwriter.ZipFile {
	if part == 0 {
		return zipwriter.NewLazyZipFile(fmt.Sprintf("%s.zip", fullPrefix))
	}
	return zipwriter.NewLazyZipFile(fmt.Sprintf("%s.%d.zip", fullPrefix, part))
}

func iterChannel[T any](ctx context.Context, ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			case item, ok := <-ch:
				if !ok {
					return
				}
				if !yield(item) {
					return
				}
			}
		}
	}
}
