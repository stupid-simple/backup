# `ssbak` = Stupid Simple Backup 

## Description

This tool allows backing up files to zip files. An automated process can be setup in order to scan target directories periodically. The process will check which files are not backed up and compress them into new zip files.

### *Why I made this*

The main reason is to have a little side project. The second reason: I wanted to have a very simple tool to backup files in my home server (mainly photos and videos). However, I want the resulting backup files (zip) to be independent from the tool generating them. Someone without a lot of tech knowledge should be able to extract those files anywhere.

## Usage

The program uses a CLI. Aside from the information below you can run `ssbak --help` to get usage info.

### `ssbak daemon -c <config file> -d <database file>` = Service mode

This will run the scheduling service. This requires a configuration file. See below.

The service uses an SQLite database where it keeps track of the backed files (assets) and destination archives.

#### Config

The configuration file uses JSON format.

Parameters:
- `sources`: A list of backup sources.
    - `source_dir`: The source directory. The directory and its subdirectories will be scanned for regular files to backup.
    - `archive_dir`: The target directory where backup archives will be generated.
    - `enable`: Whether to schedule this source.
    - `cron`: The schedule in UNIX cron format.  

Example:
```json
{
    "sources": [
        {
            "source_dir": "/backup_target",
            "archive_dir": "/zip_files_dir",
            "enable": true,
            "cron": "0 0 * * *"
        }
    ]
}
```

### `ssbak backup -s <source dir> -D <dest dir> -d <database file>` = Manually backup files

This commands scans the source directory for files and copies them into a new archive in target directory.
By default only new or modified files are copied.

The files are registered in the database.

### `ssbak restore -D <restore dir> -d <database file>` = Manually restore files

This command will restore files into the target directory. The database is used as a reference source for the files
that should be restored.

## Build

```shell
go build -o ssbak
```