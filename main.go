package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
)

var Version = "dev"

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
		kong.Description(
			fmt.Sprintf("Stupid Simple Backup. Version: %s", Version),
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupSignals(cancel)

	logger := newLogger()
	switch cli.Command() {
	case "version":
		fmt.Printf("%s\n", Version)
		cli.Exit(0)
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
