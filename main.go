package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
)

type Command struct {
	Backup struct {
		Source        string `help:"source directory path" short:"s" required:""`
		Dest          string `help:"destination directory path" short:"D" required:""`
		ArchivePrefix string `help:"archive prefix"`
		Database      string `help:"database path" short:"d" required:""`
		DryRun        bool   `help:"don't write any files, just print the output"`
	} `cmd:"" help:"Manually backup directory files."`
	Restore struct {
		Dest     string `help:"destination directory path where files will be restored" short:"D" required:""`
		Database string `help:"database path" short:"d" required:""`
		DryRun   bool   `help:"don't write any files, just print the output"`
	} `cmd:"" help:"Manually restore directory files."`
	Daemon struct {
		Config   string `help:"config file path" short:"c" required:""`
		Database string `help:"database path" short:"d" required:""`
		DryRun   bool   `help:"don't write any files, just print the output"`
	} `cmd:"" help:"Run the backup service."`
}

func newLogger() zerolog.Logger {
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false, TimeFormat: time.RFC3339}
	consoleWriter.TimeFormat = "[" + time.RFC3339 + "]"
	consoleWriter.PartsOrder = []string{
		zerolog.TimestampFieldName,
		zerolog.LevelFieldName,
		zerolog.CallerFieldName,
		zerolog.MessageFieldName,
	}

	logger := zerolog.New(consoleWriter).
		With().Timestamp().Logger()

	level := zerolog.InfoLevel
	envLevel, ok := os.LookupEnv("LOG_LEVEL")
	if ok {
		parsed, err := zerolog.ParseLevel(envLevel)
		if err != nil {
			logger.Warn().Err(err).Msg("could not parse environment variable LOG_LEVEL")
			return logger
		}
		level = parsed
	}

	return logger.Level(level)
}

func main() {
	args := Command{}
	cli := kong.Parse(&args,
		kong.Name("ssbak"),
		kong.Description("Stupid Simple Backup"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupSignals(cancel)

	logger := newLogger()
	switch cli.Command() {
	case "backup":
		err := backupCommand(ctx, args, logger)
		if err != nil {
			logger.Error().Err(err).Msg("backup error")
			cli.Exit(1)
		}
	case "restore":
		err := restoreCommand(ctx, args, logger)
		if err != nil {
			logger.Error().Err(err).Msg("restore error")
			cli.Exit(1)
		}
	case "daemon":
		err := daemonCommand(ctx, args, logger)
		if err != nil {
			logger.Error().Err(err).Msg("daemon error")
			cli.Exit(1)
		}
	default:
		panic(cli.Command())
	}
}

func setupSignals(onSignal func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		onSignal()
	}()
}
