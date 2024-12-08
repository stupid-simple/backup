package backup

type options struct {
	dryRun        bool
	archivePrefix string
}

type Option func(o *options)

func WithDryRun(dryRun bool) Option {
	return func(o *options) {
		o.dryRun = dryRun
	}
}

func WithArchivePrefix(archivePrefix string) Option {
	return func(o *options) {
		o.archivePrefix = archivePrefix
	}
}
