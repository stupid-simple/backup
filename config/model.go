package config

import "github.com/rs/zerolog"

type Config struct {
	Sources []ConfigSource `json:"sources,omitempty"`
}

type ConfigSource struct {
	SourceDir                string       `json:"source_dir"`
	ArchiveDir               string       `json:"archive_dir"`
	ArchivePrefix            string       `json:"archive_prefix"`
	ArchiveMaxFileSize       SizeArgument `json:"archive_max_file_size"`
	ArchiveIncludeLargeFiles bool         `json:"archive_include_large_files"`
	Enable                   bool         `json:"enable"`
	Schedule                 string       `json:"cron"`
}

func (s ConfigSource) MarshalZerologObject(e *zerolog.Event) {
	e.Str("source_dir", s.SourceDir)
	e.Str("archive_dir", s.ArchiveDir)
	e.Str("archive_prefix", s.ArchivePrefix)
	e.Int64("archive_max_file_size", s.ArchiveMaxFileSize.Size)
	e.Bool("archive_include_large_files", s.ArchiveIncludeLargeFiles)
	e.Bool("enable", s.Enable)
	e.Str("schedule", s.Schedule)
}
