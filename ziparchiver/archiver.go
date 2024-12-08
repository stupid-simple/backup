package ziparchiver

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/i-segura/snapsync/asset"
	"github.com/i-segura/snapsync/ziparchiver/zipwriter"
	"github.com/rs/zerolog"
)

// Define the he path to a new archive.
type PathFunc = func() string

// StoreAssets implements Archiver.
func StoreAssets(ctx context.Context, archiveFilenameFunc PathFunc, sourcePath string, assets <-chan asset.Asset, logger zerolog.Logger, opts ...StoreOption) error {
	o := storeOptions{}
	for _, applyOpts := range opts {
		applyOpts(&o)
	}

	archivePath := archiveFilenameFunc()

	logger = logger.With().Str("archive", archivePath).Logger()
	logger.Info().Msg("backing up assets")

	var wg sync.WaitGroup
	storedAssets := 0
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
			err := o.registerAssets.Register(ctx, storedCh)
			if err != nil {
				logger.Error().Err(err).Msg("could not register backup assets")
				// Drain the channel.
				for range storedCh {
				}
			}
			wg.Done()
		}()
	} else {
		onArchived = func(a asset.ArchivedAsset) {
			storedAssets++
		}
	}

	var err error
	var zipFile *zipwriter.ZipFile
	if o.dryRun {
		zipFile = zipwriter.NewNullZipFile()
	} else {
		zipFile = zipwriter.NewLazyZipFile(archivePath)
	}
	defer func() {
		if err := zipFile.Close(); err != nil {
			logger.Warn().Err(err).Msg("could not close backup file")
		}
		if storedAssets == 0 {
			err = zipFile.Delete()
			if err != nil {
				logger.Warn().Err(err).Msg("could not remove backup file")
			}
			return
		}
	}()

	return writeAssetsToZipFile(
		ctx,
		zipFile,
		sourcePath,
		assets,
		onArchived,
		logger,
	)
}

func writeAssetsToZipFile(
	ctx context.Context,
	zipFile *zipwriter.ZipFile,
	sourcePath string,
	assets <-chan asset.Asset,
	onArchived func(a asset.ArchivedAsset),
	logger zerolog.Logger) error {

	var err error
	for asset := range assets {
		if ctx.Err() != nil {
			return nil
		}

		header := &zip.FileHeader{
			UncompressedSize64: uint64(asset.Size()),
			Modified:           asset.ModTime(),
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
		assetFile.Close()
		tookSeconds := time.Since(startTime).Seconds()
		logger.Debug().Object("asset", asset).Float64("seconds", tookSeconds).Msg("archived asset")
	}()

	_, err = io.Copy(w, assetFile)
	if err != nil {
		return nil, err
	}
	return &zipAsset{
		sourcePath:       sourcePath,
		archivePath:      archivePath,
		name:             asset.Name(),
		path:             asset.Path(),
		hash:             asset.Hash(),
		modTime:          asset.ModTime(),
		uncompressedSize: asset.Size(),
	}, nil
}
