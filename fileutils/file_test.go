package fileutils_test
import (
 "os"
 "testing"

 "github.com/stupid-simple/backup/fileutils"
)

func TestExists(t *testing.T) {
 // Create a temporary file
 tmpFile, err := os.CreateTemp("", "test-file-*")
 if err != nil {
  t.Fatalf("Failed to create temporary file: %v", err)
 }
 tmpFilePath := tmpFile.Name()
 defer os.Remove(tmpFilePath) // Clean up after test
 tmpFile.Close()

 // Test cases
 tests := []struct {
  name     string
  path     string
  expected bool
 }{
  {
   name:     "existing file",
   path:     tmpFilePath,
   expected: true,
  },
  {
   name:     "non-existent file",
   path:     "non-existent-file.txt",
   expected: false,
  },
 }

 // Run tests
 for _, tc := range tests {
  t.Run(tc.name, func(t *testing.T) {
   result := fileutils.Exists(tc.path)
   if result != tc.expected {
    t.Errorf("Expected Exists(%q) = %v, got %v", tc.path, tc.expected, result)
   }
  })
 }
}