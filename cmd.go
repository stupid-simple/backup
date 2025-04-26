package main

import "github.com/stupid-simple/backup/config"

type Command struct {
	Version struct{}       `cmd:"" help:"Print version information."`
	Backup  BackupCommand  `cmd:"" help:"Manually backup directory files."`
	Restore RestoreCommand `cmd:"" help:"Manually restore directory files."`
	Copy    CopyCommand    `cmd:"" help:"Manually copy backup files into new archives."`
	Clean   CleanCommand   `cmd:"" help:"Manually clean up old backup files ."`
	Daemon  DaemonCommand  `cmd:"" help:"Run the backup service."`
}

type BackupCommand struct {
	Source            string              `help:"source directory path" short:"s" required:""`
	Dest              string              `help:"destination directory path" short:"D" required:""`
	Database          string              `help:"database path" short:"d" required:""`
	DryRun            bool                `help:"don't write any files, just print the output"`
	ArchivePrefix     string              `help:"archive prefix"`
	MaxSize           config.SizeArgument `help:"maximum stored bytes per archive in bytes"`
	IncludeLargeFiles bool                `help:"include large files in backup, will be skipped otherwise"`
}

type RestoreCommand struct {
	Dest     string `help:"destination directory path where files will be restored" short:"D" required:""`
	Database string `help:"database path" short:"d" required:""`
	DryRun   bool   `help:"don't write any files, just print the output"`
}

type CopyCommand struct {
	Source        string              `help:"only copy files backup from source directory path" short:"s"`
	Dest          string              `help:"destination directory path" short:"D" required:""`
	Database      string              `help:"database path" short:"d" required:""`
	MaxSize       config.SizeArgument `help:"maximum stored bytes per archive in bytes"`
	ArchiveLimit  int                 `help:"maximum number of archives to copy"`
	ArchivePrefix string              `help:"archive prefix"`
	DryRun        bool                `help:"don't write any files, just print the output"`
}

type CleanCommand struct {
	Source       string `help:"only clean up files backup from source directory path" short:"s" `
	Database     string `help:"database path" short:"d" required:""`
	ArchiveLimit int    `help:"maximum number of archives to clean"`
	DryRun       bool   `help:"don't write any files, just print the output"`
}

type DaemonCommand struct {
	Config   string `help:"config file path" short:"c" required:""`
	Database string `help:"database path" short:"d" required:""`
	DryRun   bool   `help:"don't write any files, just print the output"`
}
