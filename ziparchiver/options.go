package ziparchiver

import (
	"context"
	"iter"

	"github.com/stupid-simple/backup/asset"
)

type StoreOption func(o *storeOptions)

type storeOptions struct {
	dryRun            bool
	registerAssets    RegisterArchivedAssets
	onlyNewAssets     OnlyNewAssets
	maxFileBytes      int64
	includeLargeFiles bool
}

func WithDryRun(dryRun bool) StoreOption {
	return func(o *storeOptions) {
		o.dryRun = dryRun
	}
}

// The maximum number of bytes (uncompressed) to store in a single archive.
func WithMaxFileBytes(maxFileBytes int64) StoreOption {
	return func(o *storeOptions) {
		o.maxFileBytes = maxFileBytes
	}
}

// If true, files larger than maxFileBytes will be stored.
func WithIncludeLargeFiles(include bool) StoreOption {
	return func(o *storeOptions) {
		o.includeLargeFiles = include
	}
}

type RegisterArchivedAssets interface {
	Register(ctx context.Context, assets iter.Seq[asset.ArchivedAsset]) error
}

// Register the assets stored in the archive.
func WithRegisterArchivedAssets(register RegisterArchivedAssets) StoreOption {
	return func(o *storeOptions) {
		o.registerAssets = register
	}
}

type OnlyNewAssets interface {
	FindMissingAssets(ctx context.Context, from iter.Seq[asset.Asset]) (iter.Seq[asset.Asset], error)
}

func WithOnlyNewAssets(only OnlyNewAssets) StoreOption {
	return func(o *storeOptions) {
		o.onlyNewAssets = only
	}
}

type RestoreOption func(o *restoreOptions)

type restoreOptions struct {
	dryRun bool
}

func WithRestoreDryRun(dryRun bool) RestoreOption {
	return func(o *restoreOptions) {
		o.dryRun = dryRun
	}
}

type CopyArchivedOption func(o *copyArchivedOptions)

type copyArchivedOptions struct {
	dryRun         bool
	registerAssets RegisterArchivedAssets
	maxFileBytes   int64
}

func WithCopyArchivedDryRun(dryRun bool) CopyArchivedOption {
	return func(o *copyArchivedOptions) {
		o.dryRun = dryRun
	}
}

// The maximum number of bytes (uncompressed) to store in a single archive.
func WithCopyArchiveMaxFileBytes(maxFileBytes int64) CopyArchivedOption {
	return func(o *copyArchivedOptions) {
		o.maxFileBytes = maxFileBytes
	}
}

func WithCopyArchiveRegisterAssets(register RegisterArchivedAssets) CopyArchivedOption {
	return func(o *copyArchivedOptions) {
		o.registerAssets = register
	}
}
