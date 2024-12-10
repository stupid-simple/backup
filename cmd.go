package main

import "github.com/stupid-simple/backup/config"

type Command struct {
	Version struct{} `cmd:"" help:"Print version information."`
	Backup  struct {
		Source            string              `help:"source directory path" short:"s" required:""`
		Dest              string              `help:"destination directory path" short:"D" required:""`
		Database          string              `help:"database path" short:"d" required:""`
		DryRun            bool                `help:"don't write any files, just print the output"`
		ArchivePrefix     string              `help:"archive prefix"`
		MaxSize           config.SizeArgument `help:"maximum stored bytes per archive in bytes"`
		IncludeLargeFiles bool                `help:"include large files in backup, will be skipped otherwise"`
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
