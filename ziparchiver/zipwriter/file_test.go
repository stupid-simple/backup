package zipwriter_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stupid-simple/backup/fileutils"
	"github.com/stupid-simple/backup/ziparchiver/zipwriter"
)

func TestNewLazyZipFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zipwriter_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "test.zip")
	zipFile := zipwriter.NewLazyZipFile(zipPath)

	if zipFile.Path() != zipPath {
		t.Errorf("Expected path %s, got %s", zipPath, zipFile.Path())
	}

	header := &zip.FileHeader{
		Name:   "test.txt",
		Method: zip.Deflate,
	}
	writer, err := zipFile.CreateHeader(header)
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}

	content := "test content"
	_, err = writer.Write([]byte(content))
	if err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}

	err = zipFile.Close()
	if err != nil {
		t.Fatalf("Failed to close zip file: %v", err)
	}

	if !fileutils.Exists(zipPath) {
		t.Errorf("Zip file was not created at %s", zipPath)
	}

	err = zipFile.Delete()
	if err != nil {
		t.Fatalf("Failed to delete zip file: %v", err)
	}

	if fileutils.Exists(zipPath) {
		t.Errorf("Zip file was not deleted from %s", zipPath)
	}
}

func TestNewLazyZipFile_ExistingFile(t *testing.T) {
	// Create a temporary directory for our tests
	tempDir, err := os.MkdirTemp("", "zipwriter_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "existing.zip")

	// Create the file first to simulate it already existing
	_, err = os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Try to open an existing file, which should fail
	zipFile := zipwriter.NewLazyZipFile(zipPath)
	_, err = zipFile.CreateHeader(&zip.FileHeader{Name: "test.txt"})
	if err == nil {
		t.Error("Expected error when creating zip with existing file, got nil")
	}
}

func TestNewNullZipFile(t *testing.T) {
	zipFile := zipwriter.NewNullZipFile()

	// Check path
	if zipFile.Path() != "/dev/null" {
		t.Errorf("Expected path /dev/null, got %s", zipFile.Path())
	}

	// Add a file to the zip
	header := &zip.FileHeader{
		Name:   "test.txt",
		Method: zip.Deflate,
	}
	writer, err := zipFile.CreateHeader(header)
	if err != nil {
		t.Fatalf("Failed to create null zip entry: %v", err)
	}

	// Write some content
	content := "test content"
	_, err = writer.Write([]byte(content))
	if err != nil {
		t.Fatalf("Failed to write content to null device: %v", err)
	}

	// Close the zip file
	err = zipFile.Close()
	if err != nil {
		t.Fatalf("Failed to close null zip file: %v", err)
	}

	// Delete should succeed but do nothing
	err = zipFile.Delete()
	if err != nil {
		t.Fatalf("Delete on null zip file failed: %v", err)
	}
}

func TestZipFile_CloseWithoutInit(t *testing.T) {
	zipFile := zipwriter.NewLazyZipFile(filepath.Join(os.TempDir(), "nonexistent.zip"))

	// Close without initializing should not error
	err := zipFile.Close()
	if err != nil {
		t.Errorf("Expected no error when closing unopened file, got: %v", err)
	}

	// Delete without initializing should not error
	err = zipFile.Delete()
	if err != nil {
		t.Errorf("Expected no error when deleting unopened file, got: %v", err)
	}
}
