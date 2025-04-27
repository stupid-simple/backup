package ziparchiver_test

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stupid-simple/backup/asset"
	"github.com/stupid-simple/backup/fileutils"
	"github.com/stupid-simple/backup/ziparchiver"
)

type MockArchivedAsset struct {
	sourcePath  string
	archivePath string
	filePath    string
	name        string
	hash        uint64
	size        int64
	modTime     time.Time
}

func (m *MockArchivedAsset) SourcePath() string  { return m.sourcePath }
func (m *MockArchivedAsset) ArchivePath() string { return m.archivePath }
func (m *MockArchivedAsset) StoredHash() uint64  { return m.hash }
func (m *MockArchivedAsset) Path() string        { return m.filePath }
func (m *MockArchivedAsset) Name() string        { return m.name }
func (m *MockArchivedAsset) Size() int64         { return m.size }
func (m *MockArchivedAsset) ModTime() time.Time  { return m.modTime }
func (m *MockArchivedAsset) ComputeHash() (uint64, error) {
	return fileutils.ComputeFileHash(m.filePath)
}
func (m *MockArchivedAsset) MarshalZerologObject(e *zerolog.Event) {
	e.Str("path", m.filePath)
	e.Str("name", m.name)
	e.Uint64("hash", m.hash)
	e.Int64("size", m.size)
	e.Str("archive", m.archivePath)
	e.Str("source", m.sourcePath)
}

func setupTestEnvironment(t *testing.T) (string, string, string) {
	t.Helper()

	sourceDir, err := os.MkdirTemp("", "source")
	if err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	targetDir, err := os.MkdirTemp("", "target")
	if err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	archiveDir, err := os.MkdirTemp("", "archive")
	if err != nil {
		t.Fatalf("Failed to create archive directory: %v", err)
	}

	return sourceDir, targetDir, archiveDir
}

func createTestArchive(t *testing.T, sourceDir, archiveDir string) (string, map[string]*MockArchivedAsset) {
	t.Helper()

	files := []struct {
		name    string
		content string
	}{
		{"file1.txt", "This is file 1 content"},
		{"file2.txt", "This is file 2 content"},
		{"nested/file3.txt", "This is nested file 3 content"},
	}

	assets := make(map[string]*MockArchivedAsset)

	// Create the source files.
	for _, f := range files {
		fullPath := filepath.Join(sourceDir, f.name)
		dirPath := filepath.Dir(fullPath)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dirPath, err)
		}

		if err := os.WriteFile(fullPath, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	archivePath := filepath.Join(archiveDir, "test.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}
	defer func() {
		_ = archive.Close()
	}()

	zipWriter := zip.NewWriter(archive)
	defer func() {
		_ = zipWriter.Close()
	}()

	// Add files to the archive.
	for _, f := range files {
		header := &zip.FileHeader{
			Name:   f.name,
			Method: zip.Deflate,
		}
		fileInfo, err := os.Stat(filepath.Join(sourceDir, f.name))
		if err != nil {
			t.Fatalf("Failed to get file info: %v", err)
		}

		header.Modified = fileInfo.ModTime()

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			t.Fatalf("Failed to create zip entry: %v", err)
		}

		_, err = writer.Write([]byte(f.content))
		if err != nil {
			t.Fatalf("Failed to write content to zip: %v", err)
		}

		// Calculate hash
		hash, err := fileutils.ComputeHash(strings.NewReader(f.content))
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		// Create mock asset
		assets[f.name] = &MockArchivedAsset{
			sourcePath:  sourceDir,
			archivePath: archivePath,
			filePath:    filepath.Join(sourceDir, f.name),
			name:        f.name,
			hash:        hash,
			size:        int64(len(f.content)),
			modTime:     fileInfo.ModTime(),
		}
	}

	return archivePath, assets
}

// Test basic restoration functionality
func TestRestore_Basic(t *testing.T) {
	sourceDir, targetDir, archiveDir := setupTestEnvironment(t)
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(targetDir)
		_ = os.RemoveAll(archiveDir)
	}()

	archivePath, assets := createTestArchive(t, sourceDir, archiveDir)

	assetSeq := func(yield func(asset.ArchivedAsset) bool) {
		for _, a := range assets {
			// Update the file path to point to target dir instead of source.
			targetPath := filepath.Join(targetDir, a.Name())
			mockAsset := &MockArchivedAsset{
				sourcePath:  targetDir,
				archivePath: archivePath,
				filePath:    targetPath,
				name:        a.name,
				hash:        a.hash,
				size:        a.size,
				modTime:     a.modTime,
			}
			if !yield(mockAsset) {
				break
			}
		}
	}

	logger := zerolog.New(io.Discard)

	err := ziparchiver.Restore(context.Background(), assetSeq, logger)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	for _, a := range assets {
		targetPath := filepath.Join(targetDir, a.Name())

		if !fileutils.Exists(targetPath) {
			t.Errorf("Expected file %s to exist but it doesn't", targetPath)
			continue
		}

		content, err := os.ReadFile(targetPath)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", targetPath, err)
			continue
		}

		hash, err := fileutils.ComputeHash(strings.NewReader(string(content)))
		if err != nil {
			t.Errorf("Failed to compute hash for %s: %v", targetPath, err)
			continue
		}
		if hash != a.hash {
			t.Errorf("File content mismatch for %s. Expected hash %d, got %d", targetPath, a.hash, hash)
		}
	}
}

func TestRestore_DryRun(t *testing.T) {
	sourceDir, targetDir, archiveDir := setupTestEnvironment(t)
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(targetDir)
		_ = os.RemoveAll(archiveDir)
	}()

	archivePath, assets := createTestArchive(t, sourceDir, archiveDir)

	assetSeq := func(yield func(asset.ArchivedAsset) bool) {
		for _, a := range assets {
			targetPath := filepath.Join(targetDir, a.Name())
			mockAsset := &MockArchivedAsset{
				sourcePath:  targetDir,
				archivePath: archivePath,
				filePath:    targetPath,
				name:        a.name,
				hash:        a.hash,
				size:        a.size,
				modTime:     a.modTime,
			}
			if !yield(mockAsset) {
				break
			}
		}
	}

	logger := zerolog.New(io.Discard)

	err := ziparchiver.Restore(
		context.Background(),
		assetSeq,
		logger,
		ziparchiver.WithRestoreDryRun(true),
	)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Verify no files were created.
	for _, a := range assets {
		targetPath := filepath.Join(targetDir, a.Name())
		if fileutils.Exists(targetPath) {
			t.Errorf("File %s should not exist in dry run mode", targetPath)
		}
	}
}

func TestRestore_ExistingIdenticalFiles(t *testing.T) {
	sourceDir, targetDir, archiveDir := setupTestEnvironment(t)
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(targetDir)
		_ = os.RemoveAll(archiveDir)
	}()

	archivePath, assets := createTestArchive(t, sourceDir, archiveDir)

	// Create identical files in target directory.
	for _, a := range assets {
		if a.Name() != "file1.txt" {
			continue // Only create one identical file for testing.
		}

		targetPath := filepath.Join(targetDir, a.Name())
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		content, err := os.ReadFile(a.filePath)
		if err != nil {
			t.Fatalf("Failed to read source file: %v", err)
		}

		err = os.WriteFile(targetPath, content, 0644)
		if err != nil {
			t.Fatalf("Failed to write target file: %v", err)
		}

		err = os.Chtimes(targetPath, a.modTime, a.modTime)
		if err != nil {
			t.Fatalf("Failed to set file time: %v", err)
		}
	}

	assetSeq := func(yield func(asset.ArchivedAsset) bool) {
		for _, a := range assets {
			targetPath := filepath.Join(targetDir, a.Name())
			mockAsset := &MockArchivedAsset{
				sourcePath:  targetDir,
				archivePath: archivePath,
				filePath:    targetPath,
				name:        a.name,
				hash:        a.hash,
				size:        a.size,
				modTime:     a.modTime,
			}
			if !yield(mockAsset) {
				break
			}
		}
	}

	var logOutput []string
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: io.Discard, NoColor: true},
	).With().Timestamp().Logger().Hook(
		zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			logOutput = append(logOutput, fmt.Sprintf("%s: %s", level, message))
		}),
	)

	err := ziparchiver.Restore(context.Background(), assetSeq, logger)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Verify skipped files.
	var skippedFiles int
	for _, log := range logOutput {
		if log == "debug: file already present, skipping" {
			skippedFiles++
		}
	}

	if skippedFiles != 1 {
		t.Errorf("Expected 1 skipped file, got %d", skippedFiles)
	}
}

func TestRestore_ExistingModifiedFiles(t *testing.T) {
	sourceDir, targetDir, archiveDir := setupTestEnvironment(t)
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(targetDir)
		_ = os.RemoveAll(archiveDir)
	}()

	archivePath, assets := createTestArchive(t, sourceDir, archiveDir)

	for _, a := range assets {
		if a.Name() != "file1.txt" {
			continue // Only create one modified file for testing.
		}

		targetPath := filepath.Join(targetDir, a.Name())
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Write modified content to target.
		err := os.WriteFile(targetPath, []byte("Modified content"), 0644)
		if err != nil {
			t.Fatalf("Failed to write target file: %v", err)
		}

		// Set same modification time to isolate the hash checking.
		err = os.Chtimes(targetPath, a.modTime, a.modTime)
		if err != nil {
			t.Fatalf("Failed to set file time: %v", err)
		}
	}

	assetSeq := func(yield func(asset.ArchivedAsset) bool) {
		for _, a := range assets {
			targetPath := filepath.Join(targetDir, a.Name())
			mockAsset := &MockArchivedAsset{
				sourcePath:  targetDir,
				archivePath: archivePath,
				filePath:    targetPath,
				name:        a.name,
				hash:        a.hash,
				size:        a.size,
				modTime:     a.modTime,
			}
			if !yield(mockAsset) {
				break
			}
		}
	}

	var logOutput []string
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: io.Discard, NoColor: true},
	).With().Timestamp().Logger().Hook(
		zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			logOutput = append(logOutput, fmt.Sprintf("%s: %s", level, message))
		}),
	)

	err := ziparchiver.Restore(context.Background(), assetSeq, logger)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Verify skipped modified files.
	var skippedModified int
	for _, log := range logOutput {
		if log == "debug: found existing file. The file has been modified, skipping" {
			skippedModified++
		}
	}

	if skippedModified != 1 {
		t.Errorf("Expected 1 skipped modified file, got %d", skippedModified)
	}

	// Check that the file content wasn't changed.
	for _, a := range assets {
		if a.Name() != "file1.txt" {
			continue
		}

		targetPath := filepath.Join(targetDir, a.Name())
		content, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if string(content) != "Modified content" {
			t.Errorf("File content was changed when it should have been skipped")
		}
	}
}

func TestRestore_ContextCancellation(t *testing.T) {
	sourceDir, targetDir, archiveDir := setupTestEnvironment(t)
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(targetDir)
		_ = os.RemoveAll(archiveDir)
	}()

	archivePath, assets := createTestArchive(t, sourceDir, archiveDir)

	assetSeq := func(yield func(asset.ArchivedAsset) bool) {
		for _, a := range assets {
			targetPath := filepath.Join(targetDir, a.Name())
			mockAsset := &MockArchivedAsset{
				sourcePath:  targetDir,
				archivePath: archivePath,
				filePath:    targetPath,
				name:        a.name,
				hash:        a.hash,
				size:        a.size,
				modTime:     a.modTime,
			}
			if !yield(mockAsset) {
				break
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a small delay.
	go func() {
		// Give it enough time to start processing but cancel before it finishes.
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	logger := zerolog.New(io.Discard)

	err := ziparchiver.Restore(ctx, assetSeq, logger)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Check if at least some files were created
	// but not all (due to cancellation).
	var createdCount int
	for _, a := range assets {
		targetPath := filepath.Join(targetDir, a.Name())
		if fileutils.Exists(targetPath) {
			createdCount++
		}
	}

	// Since we can't predict exactly how many files will be created before cancellation,
	// we just check that some operation happened.
	t.Logf("Created %d files out of %d before cancellation", createdCount, len(assets))
}

func TestRestore_InvalidArchivePath(t *testing.T) {
	sourceDir, targetDir, archiveDir := setupTestEnvironment(t)
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(targetDir)
		_ = os.RemoveAll(archiveDir)
	}()

	_, assets := createTestArchive(t, sourceDir, archiveDir)

	// Use an invalid archive path.
	invalidArchivePath := filepath.Join(archiveDir, "nonexistent.zip")

	assetSeq := func(yield func(asset.ArchivedAsset) bool) {
		for _, a := range assets {
			// Update paths.
			targetPath := filepath.Join(targetDir, a.Name())
			mockAsset := &MockArchivedAsset{
				sourcePath:  targetDir,
				archivePath: invalidArchivePath, // Invalid path
				filePath:    targetPath,
				name:        a.name,
				hash:        a.hash,
				size:        a.size,
				modTime:     a.modTime,
			}
			if !yield(mockAsset) {
				break
			}
		}
	}

	// Collect log output to check for errors.
	var logOutput []string
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: io.Discard, NoColor: true},
	).With().Timestamp().Logger().Hook(
		zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			logOutput = append(logOutput, fmt.Sprintf("%s: %s", level, message))
		}),
	)

	err := ziparchiver.Restore(context.Background(), assetSeq, logger)
	if err != nil {
		t.Fatalf("Restore should handle errors gracefully: %v", err)
	}

	if !slices.Contains(logOutput, "warn: could not restore asset") {
		t.Errorf("Expected error logs for invalid archive path, but none found")
	}

	// Verify no files were created.
	for _, a := range assets {
		targetPath := filepath.Join(targetDir, a.Name())
		if fileutils.Exists(targetPath) {
			t.Errorf("File %s should not have been created", targetPath)
		}
	}
}

func TestRestore_TargetIsDirectory(t *testing.T) {
	sourceDir, targetDir, archiveDir := setupTestEnvironment(t)
	defer func() {
		_ = os.RemoveAll(sourceDir)
		_ = os.RemoveAll(targetDir)
		_ = os.RemoveAll(archiveDir)
	}()

	archivePath, assets := createTestArchive(t, sourceDir, archiveDir)

	// Create a directory at the target path.
	for _, a := range assets {
		if a.Name() != "file1.txt" {
			continue
		}

		dirPath := filepath.Join(targetDir, a.Name())
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
	}

	assetSeq := func(yield func(asset.ArchivedAsset) bool) {
		for _, a := range assets {
			// Update paths.
			targetPath := filepath.Join(targetDir, a.Name())
			mockAsset := &MockArchivedAsset{
				sourcePath:  targetDir,
				archivePath: archivePath,
				filePath:    targetPath,
				name:        a.name,
				hash:        a.hash,
				size:        a.size,
				modTime:     a.modTime,
			}
			if !yield(mockAsset) {
				break
			}
		}
	}

	var logOutput []string
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: io.Discard, NoColor: true},
	).With().Timestamp().Logger().Hook(
		zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			logOutput = append(logOutput, fmt.Sprintf("%s: %s", level, message))
		}),
	)

	err := ziparchiver.Restore(context.Background(), assetSeq, logger)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Check for warnings about directories.
	var dirWarnings int
	for _, log := range logOutput {
		if log == "warn: could not restore asset" {
			dirWarnings++
		}
	}

	if dirWarnings != 1 {
		t.Errorf("Expected 1 warning about target being a directory, got %d", dirWarnings)
	}
}
