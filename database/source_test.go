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

func TestBackupSource_FindMissingAssets(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	source, err := db.GetSource(ctx, "test/source/path")
	require.NoError(t, err)

	// Populate archive with one existing asset
	registerArchivedAsset(t, db, "test/source/path", "archive1", "existing/path1", 100)

	assets := []asset.Asset{
		newTestAsset("existing/path1", 100),
		newTestAsset("missing/path2", 200),
	}

	out, err := source.FindMissingAssets(ctx, slices.Values(assets))
	require.NoError(t, err)

	var missingAssets []asset.Asset
	for a := range out {
		missingAssets = append(missingAssets, a)
	}
	assert.Len(t, missingAssets, 1)
	assert.Equal(t, "missing/path2", missingAssets[0].Path())
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

	registerArchivedAsset(t, db, "test/source/path", "archive1", "archived/path1", 100)

	out, err := source.FindArchivedAssets(ctx)
	require.NoError(t, err)

	var archivedAssets []asset.ArchivedAsset
	for a := range out {
		archivedAssets = append(archivedAssets, a)
	}
	assert.Len(t, archivedAssets, 1)
	assert.Equal(t, "archived/path1", archivedAssets[0].Path())
}

func registerArchivedAsset(t *testing.T, db *database.Database, sourcePath, archivePath, assetPath string, hash int64) {
	err := db.Cli.Create(&database.ArchiveAsset{
		Archive:   database.Archive{SourcePath: sourcePath, Path: archivePath},
		Path:      assetPath,
		Hash:      hash,
		CreatedAt: time.Now(),
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

func (a *testAsset) Path() string         { return a.path }
func (a *testAsset) ComputedHash() uint64 { return a.hash }
func (a *testAsset) Name() string         { return "name_" + a.path }
func (a *testAsset) Size() int64          { return 1000 }
func (a *testAsset) ModTime() time.Time   { return time.Now() }
func (a *testAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", a.path).Uint64("hash", a.hash)
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
