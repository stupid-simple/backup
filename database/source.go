package database

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/asset"
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

// Find assets that are not in the provided sequence.
func (bs *BackupSource) FindMissingAssets(
	ctx context.Context,
	from iter.Seq[asset.Asset],
) (iter.Seq[asset.Asset], error) {
	return func(yield func(asset.Asset) bool) {
		bs.logger.Info().Msg("finding new or modified assets to backup")
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
					if ctx.Err() != nil {
						return nil
					}
					if !yield(a) {
						bs.logger.Debug().Object("asset", a).Msg("cancelled finding missing assets")
						return nil
					}
				}

				return nil
			})
	}, nil
}

func (bs *BackupSource) Register(ctx context.Context, from iter.Seq[asset.ArchivedAsset]) error {

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

func (bs *BackupSource) FindArchivedAssets(ctx context.Context, opts ...FindArchivedAssetsOptions) (iter.Seq[asset.ArchivedAsset], error) {
	o := findArchivedAssetsOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	return func(yield func(asset.ArchivedAsset) bool) {
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

			if o.archiveList != nil {
				subQuery = subQuery.Where("archive_asset.archive_path IN (?)", o.archiveList)
			}

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
				if ctx.Err() != nil {
					return
				}
				if !yield(dbAsset{&assets[i]}) {
					return
				}
			}
			offset += iterateBatchSize
		}
	}, nil
}

func (bs *BackupSource) FindArchives(ctx context.Context, opts ...FindArchivesOptions) (iter.Seq[BackupArchive], error) {
	o := findArchivesOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	return func(yield func(BackupArchive) bool) {
		offset := 0
		remaining := o.limit
		for {
			var thisBatchSize int
			if remaining > 0 {
				thisBatchSize = min(remaining, iterateBatchSize)
			} else {
				thisBatchSize = iterateBatchSize
			}

			query := bs.db.Cli.WithContext(ctx).Table("archive").
				Select("archive.path, archive.created_at, COALESCE(SUM(archive_asset.size), 0) as uncompressed_size, COUNT(archive_asset.path) as asset_count").
				Joins("LEFT JOIN archive_asset ON archive.path = archive_asset.archive_path").
				Where("archive.source_path = ?", bs.record.Path).
				Group("archive.path, archive.created_at")

			if o.onlyFullyBackedUp {
				// Find archives where all assets are also backed up in newer archives.
				query = query.Where(`
					NOT EXISTS (
						SELECT 1
						FROM archive_asset aa
						WHERE aa.archive_path = archive.path
						AND NOT EXISTS (
							SELECT 1
							FROM archive_asset aa2
							JOIN archive a2 ON aa2.archive_path = a2.path
							WHERE aa2.path = aa.path
							AND a2.source_path = archive.source_path
							AND a2.created_at > archive.created_at
						)
						LIMIT 1
					)
				`)
			}

			if o.maxSize > 0 {
				query = query.Having("COALESCE(SUM(archive_asset.size), 0) <= ?", o.maxSize)
			}

			if o.order != nil && *o.order == FindArchivesOrderBySize {
				query = query.Order("uncompressed_size")
			} else {
				query = query.Order("archive.created_at")
			}

			query = query.Limit(thisBatchSize).Offset(offset)

			type ArchiveWithSize struct {
				Path             string
				CreatedAt        time.Time
				UncompressedSize int64
				AssetCount       int
			}

			var archivesWithSize []ArchiveWithSize
			bs.db.Lock.Lock()
			err := query.Find(&archivesWithSize).Error
			bs.db.Lock.Unlock()

			if err != nil {
				bs.db.Logger.Error().Err(err).Msg("error fetching archives from database")
				return
			}
			if len(archivesWithSize) == 0 {
				return
			}
			for _, archive := range archivesWithSize {
				if ctx.Err() != nil {
					return
				}
				if !yield(BackupArchive{
					Path:       archive.Path,
					CreatedAt:  archive.CreatedAt,
					Size:       archive.UncompressedSize,
					AssetCount: archive.AssetCount,
				}) {
					return
				}
			}
			if len(archivesWithSize) < thisBatchSize {
				return
			}
			if remaining > 0 && remaining-thisBatchSize <= 0 {
				return
			}

			offset += thisBatchSize
			remaining -= thisBatchSize
		}
	}, nil
}

func (bs *BackupSource) DeleteArchives(ctx context.Context, archivePaths []string) error {
	if len(archivePaths) == 0 {
		return nil
	}

	bs.logger.Info().Strs("archives", archivePaths).Msg("deleting archives")

	bs.db.Lock.Lock()
	defer bs.db.Lock.Unlock()

	if bs.db.DryRun {
		bs.logger.Info().Strs("archives", archivePaths).Msg("would delete archives (dry run)")
		return nil
	}

	return bs.db.Cli.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First delete all assets belonging to these archives
		if err := tx.Where("archive_path IN ?", archivePaths).Delete(&ArchiveAsset{}).Error; err != nil {
			return fmt.Errorf("failed to delete archive assets: %w", err)
		}

		// Then delete the archives themselves
		if err := tx.Where("path IN ? AND source_path = ?", archivePaths, bs.record.Path).Delete(&Archive{}).Error; err != nil {
			return fmt.Errorf("failed to delete archives: %w", err)
		}

		bs.logger.Info().Int("count", len(archivePaths)).Msg("archives deleted")
		return nil
	})
}

func (bs *BackupSource) findMissingAssetsInBatches(
	ctx context.Context,
	from iter.Seq[asset.Asset],
	batchSize int,
	missing *[]asset.Asset,
	onBatch func(err error) error,
) {
	bs.logger.Info().Msg("start finding missing assets in batches")

	var countModified, countNew int

	nextAsset, stop := iter.Pull(from)
	defer stop()
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
	hasNext := true
	for hasNext {
		throttledLogger.Info().
			Int("batch_size", batchSize).
			Int("batch", len(findBatch)).
			Int("new", countNew).
			Int("modified", countModified).
			Msg("finding missing assets in batches")
		if ctx.Err() != nil {
			break
		}
		for range batchSize {
			var a asset.Asset
			a, hasNext = nextAsset()
			if !hasNext {
				bs.logger.Debug().Msg("no more assets to find")
				break
			}

			findBatch = append(findBatch, a)
			lookForPaths = append(lookForPaths, a.Path())
		}

		if len(findBatch) == 0 {
			break
		}

		results := []ArchiveAsset{}
		subQuery := bs.db.Cli.WithContext(ctx).
			Select("archive_asset.path, MAX(archive_asset.created_at) AS max_created_at").
			Joins("JOIN archive ON archive.path = archive_asset.archive_path").
			Where("archive.source_path = ? AND archive_asset.path IN ?",
				bs.record.Path, lookForPaths).
			Group("archive_asset.path").
			Table("archive_asset")

		bs.db.Lock.Lock()
		err := bs.db.Cli.WithContext(ctx).
			Select("archive_asset.*").
			Joins("JOIN (?) AS latest ON latest.path = archive_asset.path "+
				"AND latest.max_created_at = archive_asset.created_at", subQuery).
			Joins("Archive").
			Where("archive_asset.path IN ?", lookForPaths).
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
	}
}

func (bs *BackupSource) recordAssetsInBatches(
	ctx context.Context,
	from iter.Seq[asset.ArchivedAsset],
	logger zerolog.Logger,
) (int, error) {
	var countRecorded int

	nextAsset, stop := iter.Pull(from)
	defer stop()
	hasNext := true
	for hasNext {
		if ctx.Err() != nil {
			break
		}

		archiveAssets := make([]asset.ArchivedAsset, 0, iterateBatchSize)

		for range iterateBatchSize {
			var a asset.ArchivedAsset
			a, hasNext = nextAsset()
			if !hasNext {
				break
			}
			if a.SourcePath() != bs.record.Path {
				logger.Warn().Object("asset", a).Msg("skipping asset from different source")
				continue
			}

			archiveAssets = append(archiveAssets, a)
		}

		if len(archiveAssets) == 0 {
			break
		}

		logger.Debug().Int("size", len(archiveAssets)).Msg("record archive assets batch")

		bs.db.Lock.Lock()
		err := bs.db.Cli.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, a := range archiveAssets {
				if bs.db.DryRun {
					countRecorded++
					continue
				}

				if err := tx.Create(&ArchiveAsset{
					Archive: Archive{
						SourcePath: a.SourcePath(),
						Path:       a.ArchivePath(),
					},
					Path:    a.Path(),
					Size:    a.Size(),
					Hash:    int64(a.StoredHash()),
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
	}

	return countRecorded, nil
}

func isAssetModified(asset asset.Asset, archivedAsset *ArchiveAsset) (bool, error) {
	if asset.Path() != archivedAsset.Path {
		return false, fmt.Errorf("assets paths differ, %s / %s", asset.Path(), archivedAsset.Path)
	}

	if asset.ModTime().Compare(archivedAsset.ModTime) == 0 && asset.Size() == archivedAsset.Size {
		return false, nil
	}

	h, err := asset.ComputeHash()
	if err != nil {
		return false, nil
	}

	if h == uint64(archivedAsset.Hash) {
		return false, nil
	}

	return true, nil
}
