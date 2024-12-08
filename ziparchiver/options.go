package ziparchiver

import (
	"context"

	"github.com/i-segura/snapsync/asset"
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
func WithIncludeLargeFiles(skipLargeFiles bool) StoreOption {
	return func(o *storeOptions) {
		o.includeLargeFiles = skipLargeFiles
	}
}

type RegisterArchivedAssets interface {
	Register(ctx context.Context, assets <-chan asset.ArchivedAsset) error
}

// Register the assets stored in the archive.
func WithRegisterArchivedAssets(register RegisterArchivedAssets) StoreOption {
	return func(o *storeOptions) {
		o.registerAssets = register
	}
}

type OnlyNewAssets interface {
	FindMissingAssets(ctx context.Context, from <-chan asset.Asset) (<-chan asset.Asset, error)
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
