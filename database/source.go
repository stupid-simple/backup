package database

import (
	"context"

	"github.com/i-segura/snapsync/asset"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

const iterateBatchSize = 50

type BackupSource struct {
	db     *Database
	record *Source
}

func (d BackupSource) Path() string {
	return d.record.Path
}

func (d BackupSource) FindMissingAssets(ctx context.Context, from <-chan asset.Asset) (<-chan asset.Asset, error) {
	out := make(chan asset.Asset)
	go func() {
		d.db.Logger.Info().Msg("finding new or modified assets to backup")
		defer close(out)
		missing := []asset.Asset{}
		d.findMissingAssetsInBatches(ctx, from, iterateBatchSize, &missing, func(err error) error {
			if err != nil {
				d.db.Logger.Error().Err(err).Msg("could not read asset database records")
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

func (d BackupSource) Register(ctx context.Context, from <-chan asset.ArchivedAsset) error {
	logger := d.db.Logger.With().
		Str("source", d.record.Path).
		Logger()

	logger.Info().Msg("register backup assets")

	var count int
	defer func() {
		if ctx.Err() != nil {
			logger.Info().Msg("cancelled recording backup assets")
		} else if count == 0 {
			logger.Info().Msg("no backup assets recorded")
		} else {
			logger.Info().Int("recorded", count).Msg("done recording backup assets")
		}
	}()

	var err error
	count, err = d.recordAssetsInBatches(ctx, from, logger)
	if err != nil {
		return err
	}
	return nil
}

func (s BackupSource) FindArchivedAssets(ctx context.Context) (<-chan asset.ArchivedAsset, error) {
	out := make(chan asset.ArchivedAsset)

	go func() {
		defer close(out)
		offset := 0
		for {
			assets := []ArchiveAsset{}

			subQuery := s.db.Cli.WithContext(ctx).
				Select("archive_asset.path, MAX(archive_asset.created_at) AS max_created_at").
				Joins("JOIN archive ON archive.path = archive_asset.archive_path").
				Where("archive.source_path = ?", s.record.Path).
				Group("archive_asset.path").
				Order("archive_asset.created_at DESC").
				Limit(iterateBatchSize).
				Offset(offset).
				Table("archive_asset")

			s.db.Lock.Lock()
			err := s.db.Cli.
				Select("archive_asset.*").
				Joins("JOIN (?) AS latest ON latest.path = archive_asset.path "+
					"AND latest.max_created_at = archive_asset.created_at", subQuery).
				Joins("Archive").
				Order("archive_asset.created_at DESC").
				Find(&assets).Error

			s.db.Lock.Unlock()
			if err != nil {
				s.db.Logger.Error().Err(err).Msg("error fetching assets from database")
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

func (m BackupSource) findMissingAssetsInBatches(
	ctx context.Context, from <-chan asset.Asset, batchSize int, missing *[]asset.Asset, onBatch func(err error) error,
) {
	logger := m.db.Logger

	logger.Debug().Msg("find missing assets in batches")

	var countMissing int
	defer func() {
		if ctx.Err() != nil {
			logger.Info().Msg("cancelled finding assets")
		} else if countMissing == 0 {
			logger.Info().Msg("no new or modified assets found")
		} else {
			logger.Info().Int("new", countMissing).Msg("done finding new or modified assets")
		}
	}()

	findBatch := make([]asset.Asset, 0, batchSize)
	lookForPaths := make([]string, 0, batchSize)

	for {
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
		m.db.Lock.Lock()
		err := m.db.Cli.WithContext(ctx).
			Where("path in ?", lookForPaths).
			Find(&results).Error
		m.db.Lock.Unlock()
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
				m.db.Logger.Debug().Object("asset", a).Msg("asset not archived")
				countMissing++
				*missing = append(*missing, a)
				continue
			}

			if a.Hash() != uint64(archivedAsset.Hash) {
				m.db.Logger.Info().Object("asset", a).Msg("asset was modified")
				countMissing++
				*missing = append(*missing, a)
			}
		}
		if len(*missing) > 0 {
			logger.Debug().Msg("found missing assets batch")
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

func (d BackupSource) recordAssetsInBatches(ctx context.Context, from <-chan asset.ArchivedAsset, logger zerolog.Logger) (int, error) {
	var countRecorded int
	for {
		if ctx.Err() != nil {
			break
		}

		archiveAssets := make([]asset.ArchivedAsset, 0, iterateBatchSize)
		eoc := consumeN(ctx, iterateBatchSize, from, func(a asset.ArchivedAsset) {
			if a.SourcePath() != d.record.Path {
				logger.Warn().Object("asset", a).Msg("skipping asset from different source")
			}
			archiveAssets = append(archiveAssets, a)
		})

		if len(archiveAssets) == 0 {
			break
		}

		logger.Debug().Int("size", len(archiveAssets)).Msg("record archive assets batch")

		d.db.Lock.Lock()
		err := d.db.Cli.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, a := range archiveAssets {
				if err := tx.Create(&ArchiveAsset{
					Archive: Archive{
						SourcePath: a.SourcePath(),
						Path:       a.ArchivePath(),
					},
					Path:    a.Path(),
					Size:    a.Size(),
					Hash:    int64(a.Hash()),
					ModTime: a.ModTime(),
					Name:    a.Name(),
				}).Error; err != nil {
					return err
				}
				countRecorded++
			}
			return nil
		})
		d.db.Lock.Unlock()
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
