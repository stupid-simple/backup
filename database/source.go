package database

import (
	"context"
	"fmt"
	"time"

	"github.com/i-segura/snapsync/asset"
	"github.com/i-segura/snapsync/fileutils"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

const iterateBatchSize = 50

type BackupSource struct {
	db     *Database
	record *Source
	logger zerolog.Logger
}

func (bs *BackupSource) Path() string {
	return bs.record.Path
}

func (bs *BackupSource) FindMissingAssets(
	ctx context.Context,
	from <-chan asset.Asset,
) (<-chan asset.Asset, error) {
	out := make(chan asset.Asset)
	go func() {
		bs.logger.Info().Msg("finding new or modified assets to backup")
		defer close(out)
		missing := []asset.Asset{}
		bs.findMissingAssetsInBatches(
			ctx,
			from,
			iterateBatchSize,
			&missing,
			func(err error) error {
				if err != nil {
					bs.db.Logger.Error().Err(err).Msg("could not read asset database records")
					return err
				}

				for _, a := range missing {
					select {
					case <-ctx.Done():
						return nil
					case out <- a:
					}
				}

				return nil
			})
	}()

	return out, nil
}

func (bs *BackupSource) Register(ctx context.Context, from <-chan asset.ArchivedAsset) error {

	bs.logger.Info().Msg("register backup assets")

	var count int
	defer func() {
		if ctx.Err() != nil {
			bs.logger.Info().Msg("cancelled recording backup assets")
		} else if count == 0 {
			bs.logger.Info().Msg("no backup assets recorded")
		} else {
			bs.logger.Info().Int("recorded", count).Msg("done recording backup assets")
		}
	}()

	var err error
	count, err = bs.recordAssetsInBatches(ctx, from, bs.logger)
	if err != nil {
		return err
	}
	return nil
}

func (bs *BackupSource) FindArchivedAssets(ctx context.Context) (<-chan asset.ArchivedAsset, error) {
	out := make(chan asset.ArchivedAsset)

	go func() {
		defer close(out)
		offset := 0
		for {
			assets := []ArchiveAsset{}

			subQuery := bs.db.Cli.WithContext(ctx).
				Select("archive_asset.path, MAX(archive_asset.created_at) AS max_created_at").
				Joins("JOIN archive ON archive.path = archive_asset.archive_path").
				Where("archive.source_path = ?", bs.record.Path).
				Group("archive_asset.path").
				Order("archive_asset.created_at DESC").
				Limit(iterateBatchSize).
				Offset(offset).
				Table("archive_asset")

			bs.db.Lock.Lock()
			err := bs.db.Cli.
				Select("archive_asset.*").
				Joins("JOIN (?) AS latest ON latest.path = archive_asset.path "+
					"AND latest.max_created_at = archive_asset.created_at", subQuery).
				Joins("Archive").
				Order("archive_asset.created_at DESC").
				Find(&assets).Error

			bs.db.Lock.Unlock()
			if err != nil {
				bs.db.Logger.Error().Err(err).Msg("error fetching assets from database")
				return
			}
			if len(assets) == 0 {
				return
			}
			for i := range assets {
				select {
				case <-ctx.Done():
					return
				case out <- dbAsset{&assets[i]}:
				}
			}
			offset += iterateBatchSize
		}
	}()

	return out, nil
}

func (bs *BackupSource) findMissingAssetsInBatches(
	ctx context.Context,
	from <-chan asset.Asset,
	batchSize int,
	missing *[]asset.Asset,
	onBatch func(err error) error,
) {
	bs.logger.Info().Msg("start finding missing assets in batches")

	var countModified, countNew int
	defer func() {
		if ctx.Err() != nil {
			bs.logger.Info().Str("source", bs.record.Path).Msg("cancelled finding assets")
		} else if countModified+countNew == 0 {
			bs.logger.Info().Str("source", bs.record.Path).Msg("no new or modified assets found")
		} else {
			bs.logger.Info().
				Str("source", bs.record.Path).
				Int("new", countNew).
				Int("modified", countModified).
				Msg("done finding new or modified assets")
		}
	}()

	findBatch := make([]asset.Asset, 0, batchSize)
	lookForPaths := make([]string, 0, batchSize)

	throttledLogger := bs.logger.Sample(&zerolog.BurstSampler{
		Burst:  1,
		Period: 1 * time.Second,
	})
	for {
		throttledLogger.Info().
			Int("batch_size", batchSize).
			Int("batch", len(findBatch)).
			Int("new", countNew).
			Int("modified", countModified).
			Msg("finding missing assets in batches")
		if ctx.Err() != nil {
			break
		}
		eoc := consumeN(ctx, batchSize, from, func(a asset.Asset) {
			findBatch = append(findBatch, a)
			lookForPaths = append(lookForPaths, a.Path())
		})
		if len(findBatch) == 0 {
			break
		}

		results := []ArchiveAsset{}
		bs.db.Lock.Lock()
		err := bs.db.Cli.WithContext(ctx).
			Where("path in ?", lookForPaths).
			Find(&results).Error
		bs.db.Lock.Unlock()
		if err != nil {
			managedErr := onBatch(err)
			if managedErr != nil {
				return
			}
		}
		archivedByPath := map[string]*ArchiveAsset{}
		for i := range results {
			r := &results[i]
			archivedByPath[r.Path] = r
		}
		for _, a := range findBatch {
			archivedAsset, ok := archivedByPath[a.Path()]
			if !ok {
				bs.db.Logger.Debug().Object("asset", a).Msg("asset not archived")
				countNew++
				*missing = append(*missing, a)
				continue
			}

			ok, err := isAssetModified(a, archivedAsset)
			if err != nil {
				bs.db.Logger.
					Error().
					Err(err).
					Object("asset", a).
					Msg("could not compare assets. Skipping...")
				continue
			}
			if ok {
				bs.db.Logger.Info().Object("asset", a).Msg("asset was modified")
				countModified++
				*missing = append(*missing, a)
			}
		}
		if len(*missing) > 0 {
			bs.logger.Debug().Msg("found missing assets batch")
		}
		managedErr := onBatch(nil)
		if managedErr != nil {
			return
		}
		*missing = []asset.Asset{}
		findBatch = []asset.Asset{}
		lookForPaths = []string{}

		if eoc {
			break
		}
	}
}

func (bs *BackupSource) recordAssetsInBatches(
	ctx context.Context,
	from <-chan asset.ArchivedAsset,
	logger zerolog.Logger,
) (int, error) {
	var countRecorded int
	for {
		if ctx.Err() != nil {
			break
		}

		archiveAssets := make([]asset.ArchivedAsset, 0, iterateBatchSize)
		eoc := consumeN(ctx, iterateBatchSize, from, func(a asset.ArchivedAsset) {
			if a.SourcePath() != bs.record.Path {
				logger.Warn().Object("asset", a).Msg("skipping asset from different source")
			}
			archiveAssets = append(archiveAssets, a)
		})

		if len(archiveAssets) == 0 {
			break
		}

		logger.Debug().Int("size", len(archiveAssets)).Msg("record archive assets batch")

		bs.db.Lock.Lock()
		err := bs.db.Cli.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, a := range archiveAssets {
				if err := tx.Create(&ArchiveAsset{
					Archive: Archive{
						SourcePath: a.SourcePath(),
						Path:       a.ArchivePath(),
					},
					Path:    a.Path(),
					Size:    a.Size(),
					Hash:    int64(a.ComputedHash()),
					ModTime: a.ModTime(),
					Name:    a.Name(),
				}).Error; err != nil {
					return err
				}
				countRecorded++
			}
			return nil
		})
		bs.db.Lock.Unlock()
		if err != nil {
			return 0, err
		}

		logger.Debug().Msg("done record archive assets batch")

		if eoc {
			break
		}
	}

	return countRecorded, nil
}

func registerNewArchive(tx *gorm.DB, archivePath string, sourcePath string) (*Archive, error) {
	archive := Archive{
		Path:       archivePath,
		SourcePath: sourcePath,
	}
	if err := tx.Create(&archive).Error; err != nil {
		return nil, err
	}
	return &archive, nil
}

func isAssetModified(asset asset.Asset, archivedAsset *ArchiveAsset) (bool, error) {
	if asset.Path() != archivedAsset.Path {
		return false, fmt.Errorf("assets paths differ, %s / %s", asset.Path(), archivedAsset.Path)
	}

	if asset.ModTime().Compare(archivedAsset.ModTime) == 0 && asset.Size() == archivedAsset.Size {
		return false, nil
	}

	h, err := fileutils.ComputeFileHash(asset.Path())
	if err != nil {
		return false, nil
	}

	if h == uint64(archivedAsset.Hash) {
		return false, nil
	}

	return true, nil
}

// Reads size elements from the channel until it is closed or the context is cancelled.
// Returns true if there are no more elements in the channel or the context is cancelled.
func consumeN[T any](ctx context.Context, size int, ch <-chan T, on func(v T)) bool {
	for i := 0; i < size; i++ {
		select {
		case <-ctx.Done():
			return true
		case v, ok := <-ch:
			if !ok {
				return true
			}
			on(v)
		}
	}
	return false
}
