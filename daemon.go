package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/i-segura/snapsync/backup"
	"github.com/i-segura/snapsync/config"
	"github.com/i-segura/snapsync/database"
	"github.com/i-segura/snapsync/fileutils"
	"github.com/i-segura/snapsync/scheduler"
	"github.com/rs/zerolog"
)

type listenAddress struct {
	Protocol string
	Address  string
}

func daemonCommand(ctx context.Context, args Command, logger zerolog.Logger) error {
	if args.Daemon.DryRun {
		logger = logger.With().Bool("dryrun", true).Logger()
	}

	cfg, err := config.LoadFromFile(args.Daemon.Config)
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	if args.Daemon.Database == "" {
		return fmt.Errorf("no database specified")
	}

	dbCli, err := newSQLite(args.Daemon.Database, logger, args.Daemon.DryRun)
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

	addSyncJobsFromConfig(ctx, scheduler, cfg, db, logger, args.Daemon.DryRun)
	startConfigFileWatcher(ctx, args.Daemon.Config, logger, func(cfg *config.Config) {
		scheduler.RemoveJobs()
		addSyncJobsFromConfig(ctx, scheduler, cfg, db, logger, args.Daemon.DryRun)
	})

	scheduler.Start()
	defer scheduler.Stop()

	<-ctx.Done()

	return nil
}

func parseAddress(addrStr string) (listenAddress, error) {
	// Split the input string into protocol and address using "://"
	parts := strings.SplitN(addrStr, "://", 2)
	if len(parts) != 2 {
		return listenAddress{}, fmt.Errorf("invalid address format, missing protocol separator '://'")
	}

	protocol := parts[0]
	address := parts[1]

	// Validate the protocol
	if protocol != "tcp" && protocol != "unix" {
		return listenAddress{}, fmt.Errorf("invalid protocol '%s', must be 'tcp' or 'unix'", protocol)
	}

	// Validate the address based on the protocol
	if protocol == "tcp" {
		// For TCP, the address must be in host:port format
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return listenAddress{}, fmt.Errorf("invalid tcp address '%s': %v", address, err)
		}
		if host == "" || port == "" {
			return listenAddress{}, fmt.Errorf("tcp address must include both host and port")
		}
	} else {
		// For Unix, the address must be a non-empty socket file path
		if strings.TrimSpace(address) == "" {
			return listenAddress{}, fmt.Errorf("unix address must be a non-empty socket file path")
		}
	}

	return listenAddress{
		Protocol: protocol,
		Address:  address,
	}, nil
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
			Str("source", source.SourceDir).
			Str("dest", source.ArchiveDir).
			Str("schedule", source.Schedule).
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
		ctx:           ctx,
		sourcePath:    cfgSource.SourceDir,
		destPath:      cfgSource.ArchiveDir,
		dryRun:        dryRun,
		archivePrefix: cfgSource.ArchivePrefix,
		db:            db,
		logger:        logger,
	}, nil
}

func startConfigFileWatcher(ctx context.Context, cfgPath string, logger zerolog.Logger, onChanged func(cfg *config.Config)) {
	logger.Info().Str("path", cfgPath).Msg("watching config file for changes")
	watcher, err := fileutils.WatchFile(ctx, cfgPath, when(time.After(30*time.Second)), func(err error) {
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
	sourcePath    string
	destPath      string
	archivePrefix string
	ctx           context.Context
	logger        zerolog.Logger
	db            *database.Database
	dryRun        bool
}

func (b *backupJob) Run() {
	if err := backup.BackupSource(b.ctx, backup.BackupParams{
		SourcePath: b.sourcePath,
		DestPath:   b.destPath,
		DB:         b.db,
		Logger:     b.logger,
	}, backup.WithDryRun(b.dryRun), backup.WithArchivePrefix(b.archivePrefix)); err != nil {
		b.logger.Error().Err(err).Str("source", b.sourcePath).Str("dest", b.destPath).Msg("backup job failed")
	}
}
