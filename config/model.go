package config

type Config struct {
	Sources []ConfigSource `json:"sources,omitempty"`
}

type ConfigSource struct {
	SourceDir     string `json:"source_dir"`
	ArchiveDir    string `json:"archive_dir"`
	ArchivePrefix string `json:"archive_prefix"`
	Enable        bool   `json:"enable"`
	Schedule      string `json:"cron"`
}
