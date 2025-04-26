package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/config"
	"github.com/stupid-simple/backup/database"
	"github.com/stupid-simple/backup/fileutils"
	"github.com/stupid-simple/backup/scheduler"
)

func daemonCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Daemon.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	if args.Daemon.Database == "" {
		return fmt.Errorf("no database specified")
	}

	cfg, err := config.LoadFromFile(args.Daemon.Config)
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	dbCli, err := newSQLite(args.Daemon.Database, logger)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}

	db := &database.Database{
		Cli:    dbCli,
		Logger: logger,
		DryRun: args.Daemon.DryRun,
	}

	scheduler := scheduler.NewScheduler(scheduler.SchedulerParams{
		Logger: logger,
	})

	err = addSyncJobsFromConfig(ctx, scheduler, cfg, db, logger, args.Daemon.DryRun)
	if err != nil {
		return fmt.Errorf("could not add sync jobs: %w", err)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	startConfigFileWatcher(ctx, args.Daemon.Config, logger, ticker, func(cfg *config.Config) {
		scheduler.RemoveJobs()
		err := addSyncJobsFromConfig(ctx, scheduler, cfg, db, logger, args.Daemon.DryRun)
		if err != nil {
			logger.Error().Err(err).Msg("failed to add sync jobs")
		}
	})

	scheduler.Start()
	defer scheduler.Stop()

	<-ctx.Done()

	return nil
}

func addSyncJobsFromConfig(
	ctx context.Context,
	scheduler *scheduler.Scheduler,
	cfg *config.Config,
	db *database.Database,
	logger zerolog.Logger,
	dryRun bool,
) error {
	sourceDirs := make(map[string]struct{})
	destDirs := make(map[string]struct{})

	for _, source := range cfg.Sources {
		job, err := configSourceToBackupJob(ctx, &source, db, logger, dryRun)
		if err != nil {
			logger.Warn().AnErr("cause", err).Msg("skipping source")
			continue
		}

		if _, ok := sourceDirs[source.SourceDir]; ok {
			logger.Warn().Str("source", source.SourceDir).Msg("skipping duplicate source")
			continue
		}
		sourceDirs[source.SourceDir] = struct{}{}

		if _, ok := destDirs[source.ArchiveDir]; ok {
			logger.Warn().Str("dest", source.ArchiveDir).Msg("skipping duplicate destination")
			continue
		}
		destDirs[source.ArchiveDir] = struct{}{}

		if !source.Enable {
			logger.Info().Str("source", source.SourceDir).Msg("skipping disabled backup source")
			continue
		}

		if err := scheduler.AddBackupJob(ctx, source.Schedule, job); err != nil {
			logger.Error().Err(err).Str("source", source.SourceDir).Msg("could not add backup job")
			continue
		}

		logger.Info().
			Object("source", source).
			Msg("added sync job")
	}
	return nil
}

func configSourceToBackupJob(
	ctx context.Context,
	cfgSource *config.ConfigSource,
	db *database.Database,
	logger zerolog.Logger,
	dryRun bool,
) (scheduler.BackupJob, error) {
	if cfgSource.SourceDir == "" {
		return nil, fmt.Errorf("source must have a directory")
	}
	if cfgSource.ArchiveDir == "" {
		return nil, fmt.Errorf("source must have a destination")
	}
	if cfgSource.Schedule == "" {
		return nil, fmt.Errorf("source must have a schedule")
	}

	return &backupJob{
		ctx:               ctx,
		sourcePath:        cfgSource.SourceDir,
		destPath:          cfgSource.ArchiveDir,
		dryRun:            dryRun,
		archivePrefix:     cfgSource.ArchivePrefix,
		maxFileBytes:      cfgSource.ArchiveMaxFileSize.Size,
		includeLargeFiles: cfgSource.ArchiveIncludeLargeFiles,
		db:                db,
		logger:            logger,
	}, nil
}

func startConfigFileWatcher(ctx context.Context, cfgPath string, logger zerolog.Logger, ticker *time.Ticker, onChanged func(cfg *config.Config)) {
	logger.Info().Str("path", cfgPath).Msg("watching config file for changes")
	watcher, err := fileutils.WatchFile(ctx, cfgPath, when(ticker.C), func(err error) {
		logger.Error().Err(err).Msg("could not watch config file")
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not watch config file")
		return
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-watcher:
				logger.Info().Str("path", cfgPath).Msg("config file changed, reloading")

				cfg, err := config.LoadFromFile(cfgPath)
				if err != nil {
					logger.Error().Err(err).Msg("could not load config")
					break
				}

				onChanged(cfg)
			}
		}
	}()
}

func when[T any](ch <-chan T) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		defer close(out)
		for range ch {
			out <- struct{}{}
		}
	}()
	return out
}

type backupJob struct {
	sourcePath        string
	destPath          string
	archivePrefix     string
	maxFileBytes      int64
	includeLargeFiles bool
	ctx               context.Context
	logger            zerolog.Logger
	db                *database.Database
	dryRun            bool
}

func (b *backupJob) Run() {
	err := backupFiles(
		b.ctx,
		backupParams{
			sourcePath:        b.sourcePath,
			destPath:          b.destPath,
			archivePrefix:     b.archivePrefix,
			maxFileBytes:      b.maxFileBytes,
			includeLargeFiles: b.includeLargeFiles,
			db:                b.db,
			dryRun:            b.dryRun,
			logger:            b.logger,
		},
	)
	if err != nil {
		b.logger.Error().Err(err).Str("source", b.sourcePath).Str("dest", b.destPath).Msg("backup job failed")
	}
}
