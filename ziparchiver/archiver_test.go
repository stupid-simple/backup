package ziparchiver_test

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stupid-simple/backup/asset"
	"github.com/stupid-simple/backup/ziparchiver"
)

// MockAsset implements asset.Asset
type MockAsset struct {
	path    string
	name    string
	size    int64
	modTime time.Time
	content string
}

func (m *MockAsset) Path() string       { return m.path }
func (m *MockAsset) Name() string       { return m.name }
func (m *MockAsset) Size() int64        { return m.size }
func (m *MockAsset) ModTime() time.Time { return m.modTime }
func (m *MockAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", m.path)
	e.Str("name", m.name)
	e.Int64("size", m.size)
}

// MockArchivedAssetRegistry implements ziparchiver.RegisterArchivedAssets
type MockArchivedAssetRegistry struct {
	assets []asset.ArchivedAsset
}

func (r *MockArchivedAssetRegistry) Register(ctx context.Context, assets iter.Seq[asset.ArchivedAsset]) error {
	for a := range assets {
		r.assets = append(r.assets, a)
	}
	return nil
}

// MockOnlyNewAssets implements ziparchiver.OnlyNewAssets
type MockOnlyNewAssets struct {
	filter func(asset.Asset) bool
}

func (f *MockOnlyNewAssets) FindMissingAssets(ctx context.Context, from iter.Seq[asset.Asset]) (iter.Seq[asset.Asset], error) {
	return func(yield func(asset.Asset) bool) {
		for a := range from {
			if f.filter(a) {
				if !yield(a) {
					break
				}
			}
		}
	}, nil
}

// Helper to create test assets
func createTestAssets(t *testing.T, baseDir string, count int) []asset.Asset {
	assets := make([]asset.Asset, 0, count)

	for i := range count {
		content := fmt.Sprintf("Content for file %d", i)
		name := fmt.Sprintf("file%d.txt", i)
		path := filepath.Join(baseDir, name)

		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)

		info, err := os.Stat(path)
		require.NoError(t, err)

		asset, err := asset.NewFromFS(path, info)
		require.NoError(t, err)

		assets = append(assets, asset)
	}

	return assets
}

func TestStoreAssets_Basic(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	assets := createTestAssets(t, sourceDir, 3)

	assetSeq := func(yield func(asset.Asset) bool) {
		for _, a := range assets {
			if !yield(a) {
				break
			}
		}
	}

	logger := zerolog.New(io.Discard)

	// Store assets
	err := ziparchiver.StoreAssets(
		context.Background(),
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir, Prefix: "backup-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
	)

	require.NoError(t, err)

	// Verify backup file was created
	files, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)

	// Create registry
	registry := &MockArchivedAssetRegistry{}

	// Store assets with registry
	err = ziparchiver.StoreAssets(
		context.Background(),
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir, Prefix: "backup-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
		ziparchiver.WithRegisterArchivedAssets(registry),
	)

	require.NoError(t, err)

	// Verify registry received all assets
	assert.Len(t, registry.assets, 3, "Registry should receive all assets")

	// Verify correct asset attributes in registry
	for _, a := range registry.assets {
		assert.Equal(t, sourceDir, a.SourcePath(), "SourcePath should match")
		assert.NotEmpty(t, a.ArchivePath(), "ArchivePath should be set")
		assert.NotZero(t, a.ComputedHash(), "Hash should be computed")
	}
}

func TestStoreAssets_WithFiltering(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create test assets with different sizes.
	assets := []asset.Asset{}

	for i := range 3 {
		content := strings.Repeat("A", (i+1)*100) // 100, 200, 300 bytes.
		name := fmt.Sprintf("file%d.txt", i)
		path := filepath.Join(sourceDir, name)

		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)

		info, err := os.Stat(path)
		require.NoError(t, err)

		asset, err := asset.NewFromFS(path, info)
		require.NoError(t, err)

		assets = append(assets, asset)
	}

	assetSeq := func(yield func(asset.Asset) bool) {
		for _, a := range assets {
			if !yield(a) {
				break
			}
		}
	}

	// Filter that only passes files smaller than 200 bytes.
	filter := &MockOnlyNewAssets{
		filter: func(a asset.Asset) bool {
			return a.Size() < 200
		},
	}

	logger := zerolog.New(io.Discard)

	// Store assets with filtering.
	err := ziparchiver.StoreAssets(
		context.Background(),
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir, Prefix: "backup-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
		ziparchiver.WithOnlyNewAssets(filter),
	)

	require.NoError(t, err)

	// Verify zip contents - should only contain one file.
	files, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.Len(t, files, 1, "Should create a single zip file")

	r, err := zip.OpenReader(filepath.Join(destDir, files[0].Name()))
	require.NoError(t, err)
	defer r.Close()

	assert.Len(t, r.File, 1, "Zip should contain only small assets")
	assert.Equal(t, "file0.txt", filepath.Base(r.File[0].Name), "Should only contain the smallest file")
}

func TestStoreAssets_WithMaxFileSize(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create test assets with different sizes.
	assets := []asset.Asset{}

	for i := range 3 {
		content := strings.Repeat("A", (i+1)*1000) // 1KB, 2KB, 3KB.
		name := fmt.Sprintf("file%d.txt", i)
		path := filepath.Join(sourceDir, name)

		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)

		info, err := os.Stat(path)
		require.NoError(t, err)

		asset, err := asset.NewFromFS(path, info)
		require.NoError(t, err)

		assets = append(assets, asset)
	}

	assetSeq := func(yield func(asset.Asset) bool) {
		for _, a := range assets {
			if !yield(a) {
				break
			}
		}
	}

	logger := zerolog.New(io.Discard)

	// Store assets with max file size of 2.5KB (should create two zip files).
	err := ziparchiver.StoreAssets(
		context.Background(),
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir, Prefix: "backup-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
		ziparchiver.WithMaxFileBytes(2500),
	)

	require.NoError(t, err)

	// Verify multiple zip files were created.
	files, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.Len(t, files, 2, "Should create two zip files")

	// Verify naming convention for multiple parts.
	hasMainFile := false
	hasPartFile := false

	for _, f := range files {
		if strings.Contains(f.Name(), ".1.zip") {
			hasPartFile = true
		} else if strings.HasSuffix(f.Name(), ".zip") && !strings.Contains(f.Name(), ".1.zip") {
			hasMainFile = true
		}
	}

	assert.True(t, hasMainFile, "Should have main zip file")
	assert.True(t, hasPartFile, "Should have part zip file")
}

func TestStoreAssets_WithLargeFileHandling(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a small file and a "large" file.
	smallContent := "Small file content"
	largeContent := strings.Repeat("A", 2000) // 2KB.

	smallPath := filepath.Join(sourceDir, "small.txt")
	largePath := filepath.Join(sourceDir, "large.txt")

	err := os.WriteFile(smallPath, []byte(smallContent), 0644)
	require.NoError(t, err)

	err = os.WriteFile(largePath, []byte(largeContent), 0644)
	require.NoError(t, err)

	smallInfo, err := os.Stat(smallPath)
	require.NoError(t, err)

	largeInfo, err := os.Stat(largePath)
	require.NoError(t, err)

	smallAsset, err := asset.NewFromFS(smallPath, smallInfo)
	require.NoError(t, err)

	largeAsset, err := asset.NewFromFS(largePath, largeInfo)
	require.NoError(t, err)

	assets := []asset.Asset{smallAsset, largeAsset}

	assetSeq := func(yield func(asset.Asset) bool) {
		for _, a := range assets {
			if !yield(a) {
				break
			}
		}
	}

	logger := zerolog.New(io.Discard)

	// Test 1: Store with max size 1KB, exclude large files (default).
	err = ziparchiver.StoreAssets(
		context.Background(),
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir, Prefix: "exclude-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
		ziparchiver.WithMaxFileBytes(1000),
	)

	require.NoError(t, err)

	// Verify only the small file was included.
	files, err := os.ReadDir(destDir)
	require.NoError(t, err)

	excludeZipPath := ""
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "exclude-") {
			excludeZipPath = filepath.Join(destDir, f.Name())
			break
		}
	}

	r, err := zip.OpenReader(excludeZipPath)
	require.NoError(t, err)
	defer r.Close()

	assert.Len(t, r.File, 1, "Should only include the small file")
	assert.Equal(t, "small.txt", filepath.Base(r.File[0].Name), "Should be the small file")

	// Test 2: Store with max size 1KB, but include large files.
	destDir2 := t.TempDir()

	err = ziparchiver.StoreAssets(
		context.Background(),
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir2, Prefix: "include-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
		ziparchiver.WithMaxFileBytes(1000),
		ziparchiver.WithIncludeLargeFiles(true),
	)

	require.NoError(t, err)

	// Verify both files were included.
	files, err = os.ReadDir(destDir2)
	require.NoError(t, err)
	assert.Len(t, files, 2, "Should create two zip files")

	// Count total files in all zips.
	totalFiles := 0
	for _, f := range files {
		zipPath := filepath.Join(destDir2, f.Name())
		r, err := zip.OpenReader(zipPath)
		require.NoError(t, err)
		totalFiles += len(r.File)
		r.Close()
	}

	assert.Equal(t, 2, totalFiles, "Should include both files across all zips")
}

func TestStoreAssets_DryRun(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	assets := createTestAssets(t, sourceDir, 3)

	assetSeq := func(yield func(asset.Asset) bool) {
		for _, a := range assets {
			if !yield(a) {
				break
			}
		}
	}

	logger := zerolog.New(io.Discard)

	// Register should still work in dry run.
	registry := &MockArchivedAssetRegistry{}

	// Store assets with dry run.
	err := ziparchiver.StoreAssets(
		context.Background(),
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir, Prefix: "backup-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
		ziparchiver.WithDryRun(true),
		ziparchiver.WithRegisterArchivedAssets(registry),
	)

	require.NoError(t, err)

	// Verify no files were created.
	files, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.Len(t, files, 0, "Dry run should not create any files")

	// Verify registry still received the assets.
	assert.Len(t, registry.assets, 3, "Registry should still receive assets in dry run")
}

func TestStoreAssets_Cancellation(t *testing.T) {
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create many test assets to give us time to cancel.
	count := 100
	assets := make([]asset.Asset, 0, count)

	for i := range count {
		content := strings.Repeat("A", 10000) // 10KB each to slow things down.
		name := fmt.Sprintf("file%d.txt", i)
		path := filepath.Join(sourceDir, name)

		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)

		info, err := os.Stat(path)
		require.NoError(t, err)

		a, err := asset.NewFromFS(path, info)
		require.NoError(t, err)

		assets = append(assets, a)
	}

	assetSeq := func(yield func(asset.Asset) bool) {
		for _, a := range assets {
			time.Sleep(10 * time.Millisecond) // Slow down iteration.
			if !yield(a) {
				break
			}
		}
	}

	logger := zerolog.New(io.Discard)

	// Create a context we'll cancel.
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short time.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := ziparchiver.StoreAssets(
		ctx,
		sourceDir,
		ziparchiver.ArchiveDescriptor{Dir: destDir, Prefix: "backup-"},
		iter.Seq[asset.Asset](assetSeq),
		logger,
	)

	require.NoError(t, err, "Cancellation should not return an error")

	// Verify a partial zip file was created.
	files, err := os.ReadDir(destDir)
	require.NoError(t, err)
	assert.Len(t, files, 1, "Should create a zip file")

	// Verify the zip contains fewer than all assets.
	r, err := zip.OpenReader(filepath.Join(destDir, files[0].Name()))
	require.NoError(t, err)
	defer r.Close()

	assert.Less(t, len(r.File), count, "Should contain fewer than all assets due to cancellation")
}
