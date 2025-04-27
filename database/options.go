package database

type findArchivesOptions struct {
	limit             int
	order             *FindArchivesOrderBy
	maxSize           int64
	onlyFullyBackedUp bool
}

type FindArchivesOptions func(*findArchivesOptions)

// Limit the number of archives returned.
func WithFindArchivesLimit(limit int) FindArchivesOptions {
	return func(o *findArchivesOptions) {
		o.limit = limit
	}
}

type FindArchivesOrderBy string

const (
	// Order by size, smallest first.
	FindArchivesOrderBySize FindArchivesOrderBy = "size"
)

// Return the archives in a specific order.
func WithFindArchivesOrderBy(order FindArchivesOrderBy) FindArchivesOptions {
	return func(o *findArchivesOptions) {
		o.order = &order
	}
}

// Limit the maximum uncompressed size of archives returned.
func WithFindArchivesMaxUncompressedSize(maxSize int64) FindArchivesOptions {
	return func(o *findArchivesOptions) {
		o.maxSize = maxSize
	}
}

// Find only archives where all assets
// are also backed up in newer archives.
func WithFindArchivesOnlyFullyBackedUp() FindArchivesOptions {
	return func(o *findArchivesOptions) {
		o.onlyFullyBackedUp = true
	}
}
