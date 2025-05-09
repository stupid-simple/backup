package database_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stupid-simple/backup/asset"
	"github.com/stupid-simple/backup/database"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Helper to set up an in-memory SQLite database
func setupTestDB(t *testing.T) *database.Database {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	require.NoError(t, err)

	// Perform database migrations
	err = gormDB.AutoMigrate(&database.Source{}, &database.Archive{}, &database.ArchiveAsset{})
	require.NoError(t, err)

	return &database.Database{
		Lock:   sync.Mutex{},
		Cli:    gormDB,
		Logger: zerolog.Nop(),
		DryRun: false,
	}
}

func TestDatabase_GetSource(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Test creating and fetching a source
	path := "test/source/path"
	source, err := db.GetSource(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, path, source.Path())

	// Ensure GetSource is idempotent
	source2, err := db.GetSource(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, source.Path(), source2.Path())
}

func TestBackupSource_FindMissingAssetsWithVersions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)

	registerArchivedAsset(t, db, "test/source/path", "archive1", "versioned/path", 100, oldTime)
	registerArchivedAsset(t, db, "test/source/path", "archive2", "versioned/path", 200, newTime)

	// Different scenarios to test
	testCases := []struct {
		name            string
		assetToCheck    asset.Asset
		shouldBeMissing bool
	}{
		{
			name:            "unmodified asset matching latest version",
			assetToCheck:    newTestAsset("versioned/path", 200), // Matches newest version hash
			shouldBeMissing: false,
		},
		{
			name:            "asset modified from latest version",
			assetToCheck:    newTestAsset("versioned/path", 300), // Different from newest version hash
			shouldBeMissing: true,
		},
		{
			name:            "asset matching old version but not latest",
			assetToCheck:    newTestAsset("versioned/path", 100), // Matches old version but not newest
			shouldBeMissing: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assets := []asset.Asset{tc.assetToCheck}

			out, err := source.FindMissingAssets(ctx, slices.Values(assets))
			require.NoError(t, err)

			var missingAssets []asset.Asset
			for a := range out {
				missingAssets = append(missingAssets, a)
			}

			if tc.shouldBeMissing {
				assert.Len(t, missingAssets, 1, "Expected asset to be marked as missing")
				if len(missingAssets) > 0 {
					assert.Equal(t, tc.assetToCheck.Path(), missingAssets[0].Path())
				}
			} else {
				assert.Len(t, missingAssets, 0, "Expected asset to NOT be marked as missing")
			}
		})
	}
}

func TestBackupSource_FindMissingAssetsComplex(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)

	registerArchivedAsset(t, db, "test/source/path", "archive1", "path1", 100, oldTime)
	registerArchivedAsset(t, db, "test/source/path", "archive2", "path1", 200, newTime) // Newer version of path1
	registerArchivedAsset(t, db, "test/source/path", "archive1", "path2", 300, oldTime)
	registerArchivedAsset(t, db, "test/source/path", "archive1", "path3", 400, oldTime)

	assets := []asset.Asset{
		newTestAsset("path1", 200), // Matches latest version
		newTestAsset("path2", 350), // Modified from only version
		newTestAsset("path3", 400), // Matches only version
		newTestAsset("path4", 500), // New path not in database
	}

	out, err := source.FindMissingAssets(ctx, slices.Values(assets))
	require.NoError(t, err)

	var missingAssets []asset.Asset
	for a := range out {
		missingAssets = append(missingAssets, a)
	}

	// Should find 2 missing assets: path2 (modified) and path4 (new)
	assert.Len(t, missingAssets, 2)

	// Collect paths of missing assets
	missingPaths := make([]string, len(missingAssets))
	for i, a := range missingAssets {
		missingPaths[i] = a.Path()
	}

	// Verify the right assets were identified as missing
	assert.Contains(t, missingPaths, "path2")
	assert.Contains(t, missingPaths, "path4")
	assert.NotContains(t, missingPaths, "path1")
	assert.NotContains(t, missingPaths, "path3")
}

func TestBackupSource_Register(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)

	archivedAssets := []asset.ArchivedAsset{
		newTestArchivedAsset("test/source/path", "archive1", "path1", 123),
		newTestArchivedAsset("test/source/path", "archive2", "path2", 456),
	}

	err = source.Register(ctx, slices.Values(archivedAssets))
	require.NoError(t, err)

	var assets []database.ArchiveAsset
	err = db.Cli.Find(&assets).Error
	require.NoError(t, err)
	assert.Len(t, assets, 2)
	assert.Equal(t, "path1", assets[0].Path)
}

func TestBackupSource_FindArchivedAssets(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)
	now := time.Now()

	registerArchivedAsset(t, db, "test/source/path", "archive1", "archived/path1", 100, now)

	out, err := source.FindArchivedAssets(ctx)
	require.NoError(t, err)

	var archivedAssets []asset.ArchivedAsset
	for a := range out {
		archivedAssets = append(archivedAssets, a)
	}
	assert.Len(t, archivedAssets, 1)
	assert.Equal(t, "archived/path1", archivedAssets[0].Path())
}

func TestBackupSource_DeleteArchives(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)
	now := time.Now()

	// Create some test archives with assets
	registerArchivedAsset(t, db, "test/source/path", "archive1", "path1", 100, now)
	registerArchivedAsset(t, db, "test/source/path", "archive1", "path2", 200, now)
	registerArchivedAsset(t, db, "test/source/path", "archive2", "path3", 300, now)
	registerArchivedAsset(t, db, "test/source/path", "archive3", "path4", 400, now)

	// Delete archive1 and archive2
	err = source.DeleteArchive(ctx, "archive1")
	require.NoError(t, err)
	err = source.DeleteArchive(ctx, "archive2")
	require.NoError(t, err)

	// Verify archives were deleted
	var archives []database.Archive
	err = db.Cli.Find(&archives).Error
	require.NoError(t, err)
	assert.Len(t, archives, 1)
	assert.Equal(t, "archive3", archives[0].Path)

	// Verify assets were deleted
	var assets []database.ArchiveAsset
	err = db.Cli.Find(&assets).Error
	require.NoError(t, err)
	assert.Len(t, assets, 1)
	assert.Equal(t, "path4", assets[0].Path)
}

func TestBackupSource_FindArchives(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)

	// Create test archives with different sizes
	now := time.Now()
	err = db.Cli.Create(&database.Archive{
		Path:       "archive1",
		SourcePath: "test/source/path",
		CreatedAt:  now.Add(-time.Hour),
	}).Error
	require.NoError(t, err)

	err = db.Cli.Create(&database.Archive{
		Path:       "archive2",
		SourcePath: "test/source/path",
		CreatedAt:  now,
	}).Error
	require.NoError(t, err)

	// Add assets with specific sizes
	err = db.Cli.Create(&database.ArchiveAsset{
		ArchivePath: "archive1",
		Path:        "path1",
		Size:        3000,
	}).Error
	require.NoError(t, err)

	err = db.Cli.Create(&database.ArchiveAsset{
		ArchivePath: "archive2",
		Path:        "path2",
		Size:        1000,
	}).Error
	require.NoError(t, err)

	// Test basic find archives
	archives, err := source.FindArchives(ctx)
	require.NoError(t, err)

	var results []database.BackupArchive
	for a := range archives {
		results = append(results, a)
	}
	assert.Len(t, results, 2)

	// Test with limit
	archives, err = source.FindArchives(ctx, database.WithFindArchivesLimit(1))
	require.NoError(t, err)

	results = nil
	for a := range archives {
		results = append(results, a)
	}
	assert.Len(t, results, 1)

	// Test with order by size
	orderBySize := database.FindArchivesOrderBySize
	archives, err = source.FindArchives(ctx, database.WithFindArchivesOrderBy(orderBySize))
	require.NoError(t, err)

	results = nil
	for a := range archives {
		results = append(results, a)
	}
	assert.Len(t, results, 2)
	// First archive should be the smaller one (archive2 with 1000 bytes)
	assert.Equal(t, "archive2", results[0].Path)

	// Test with max size
	archives, err = source.FindArchives(ctx, database.WithFindArchivesMaxUncompressedSize(2000))
	require.NoError(t, err)

	results = nil
	for a := range archives {
		results = append(results, a)
	}
	assert.Len(t, results, 1)
	assert.Equal(t, "archive2", results[0].Path)
}

func TestDatabase_IterSources(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create multiple sources
	_, err := db.GetSource(ctx, "source1")
	require.NoError(t, err)
	_, err = db.GetSource(ctx, "source2")
	require.NoError(t, err)
	_, err = db.GetSource(ctx, "source3")
	require.NoError(t, err)

	// Test iterating sources
	sources, err := db.IterSources(ctx)
	require.NoError(t, err)

	var sourcePaths []string
	for source := range sources {
		sourcePaths = append(sourcePaths, source.Path())
	}

	assert.Len(t, sourcePaths, 3)
	assert.Contains(t, sourcePaths, "source1")
	assert.Contains(t, sourcePaths, "source2")
	assert.Contains(t, sourcePaths, "source3")
}

func TestBackupSource_FindArchivesOnlyFullyBackedUp(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)

	// Configure different creation times for archives
	now := time.Now()
	older := now.Add(-time.Hour)
	oldest := now.Add(-2 * time.Hour)

	// Create archives with specific creation times
	err = db.Cli.Create(&database.Archive{
		Path:       "archive1",
		SourcePath: "test/source/path",
		CreatedAt:  oldest,
	}).Error
	require.NoError(t, err)

	err = db.Cli.Create(&database.Archive{
		Path:       "archive2",
		SourcePath: "test/source/path",
		CreatedAt:  older,
	}).Error
	require.NoError(t, err)

	err = db.Cli.Create(&database.Archive{
		Path:       "archive3",
		SourcePath: "test/source/path",
		CreatedAt:  now,
	}).Error
	require.NoError(t, err)

	// Create assets in these archives
	// archive1 has path1 and path2
	// archive2 has path2 and path3
	// archive3 has path1 and path3
	// This means archive1 is fully backed up (path1 in archive3, path2 in archive2)
	// but archive2 is not (path2 is not in archive3)

	registerArchivedAsset(t, db, "test/source/path", "archive1", "path1", 100, oldest)
	registerArchivedAsset(t, db, "test/source/path", "archive1", "path2", 200, oldest)
	registerArchivedAsset(t, db, "test/source/path", "archive2", "path2", 200, older)
	registerArchivedAsset(t, db, "test/source/path", "archive2", "path3", 300, older)
	registerArchivedAsset(t, db, "test/source/path", "archive3", "path1", 100, now)
	registerArchivedAsset(t, db, "test/source/path", "archive3", "path3", 300, now)

	// Test for fully backed up archives
	archives, err := source.FindArchives(ctx, database.WithFindArchivesOnlyFullyBackedUp())
	require.NoError(t, err)

	var results []database.BackupArchive
	for a := range archives {
		results = append(results, a)
	}

	// Only archive1 should be fully backed up
	assert.Len(t, results, 1)
	assert.Equal(t, "archive1", results[0].Path)
}

func registerArchivedAsset(t *testing.T, db *database.Database, sourcePath, archivePath, assetPath string, hash int64, createdAt time.Time) {
	err := db.Cli.Create(&database.ArchiveAsset{
		Archive:   database.Archive{SourcePath: sourcePath, Path: archivePath},
		Path:      assetPath,
		Hash:      hash,
		CreatedAt: createdAt,
		ModTime:   createdAt,
	}).Error
	require.NoError(t, err)
}

// Helper function to create a mock Asset
func newTestAsset(path string, hash uint64) asset.Asset {
	return &testAsset{path: path, hash: hash}
}

// Helper function to create a mock ArchivedAsset
func newTestArchivedAsset(sourcePath, archivePath, path string, hash uint64) asset.ArchivedAsset {
	return &testArchivedAsset{
		testAsset:   testAsset{path: path, hash: hash},
		sourcePath:  sourcePath,
		archivePath: archivePath,
	}
}

// testAsset implements asset.Asset
type testAsset struct {
	path string
	hash uint64
}

func (a *testAsset) Path() string       { return a.path }
func (a *testAsset) StoredHash() uint64 { return a.hash }
func (a *testAsset) Name() string       { return "name_" + a.path }
func (a *testAsset) Size() int64        { return 1000 }
func (a *testAsset) ModTime() time.Time { return time.Now() }
func (a *testAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", a.path).Uint64("hash", a.hash)
}
func (a *testAsset) ComputeHash() (uint64, error) {
	return a.hash, nil
}

// testArchivedAsset implements asset.ArchivedAsset
type testArchivedAsset struct {
	testAsset
	sourcePath  string
	archivePath string
}

func (a *testArchivedAsset) SourcePath() string  { return a.sourcePath }
func (a *testArchivedAsset) ArchivePath() string { return a.archivePath }
func (a *testArchivedAsset) ArchivedSize() int64 { return 100 }
