package ziparchiver

import (
	"context"

	"github.com/i-segura/snapsync/asset"
)

type StoreOption func(o *storeOptions)

type storeOptions struct {
	dryRun         bool
	registerAssets RegisterArchivedAssets
	onlyNewAssets  OnlyNewAssets
}

func WithDryRun(dryRun bool) StoreOption {
	return func(o *storeOptions) {
		o.dryRun = dryRun
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
