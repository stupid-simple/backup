package asset_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stupid-simple/backup/asset"
)

func TestScanDirectory(t *testing.T) {
	// Setup temp directory for tests
	tempDir, err := os.MkdirTemp("", "asset-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a few test files
	testFiles := []string{
		"file1.txt",
		"file2.jpg",
		"file3.png",
	}

	for _, filename := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(subDir, "subfile.txt"), []byte("subdir content"), 0644)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name     string
		dir      string
		setupCtx func() (context.Context, context.CancelFunc)
		expected int
		wantErr  bool
	}{
		{
			name: "successfully scan directory",
			dir:  tempDir,
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			expected: 4, // 3 files in root dir + 1 in subdir
			wantErr:  false,
		},
		{
			name: "cancelled context",
			dir:  tempDir,
			setupCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Pre-cancel the context
				return ctx, cancel
			},
			expected: 0,
			wantErr:  false, // The function returns nil error even with cancelled context
		},
		{
			name: "non-existent directory",
			dir:  filepath.Join(tempDir, "nonexistent"),
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			expected: 0,
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := zerolog.New(zerolog.NewTestWriter(t))
			ctx, cancel := tc.setupCtx()
			defer cancel()

			seq, err := asset.ScanDirectory(ctx, tc.dir, logger)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Count assets in the sequence
			count := 0
			for a := range seq {
				count++
				assert.NotEmpty(t, a.Path)
				assert.NotZero(t, a.Size)
				assert.NotZero(t, a.ModTime)
			}

			assert.Equal(t, tc.expected, count)
		})
	}
}

func TestScanDirectoryErrors(t *testing.T) {
	// Mock a directory with permission issues
	tempDir, err := os.MkdirTemp("", "asset-test-perm-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a file we can't read
	restrictedFile := filepath.Join(tempDir, "restricted.txt")
	err = os.WriteFile(restrictedFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Make it unreadable if we're not on Windows
	// This test will be skipped on Windows where permission model is different
	if os.Getenv("OS") != "Windows_NT" {
		err = os.Chmod(restrictedFile, 0000)
		require.NoError(t, err)

		logger := zerolog.New(zerolog.NewTestWriter(t))
		ctx := context.Background()

		seq, err := asset.ScanDirectory(ctx, tempDir, logger)
		assert.NoError(t, err) // The function should not return an error

		count := 0
		for a := range seq {
			count++
			t.Logf("Found asset: %s", a.Path())
		}

		// We shouldn't find the restricted file as a valid asset
		assert.Zero(t, count)
	} else {
		t.Skip("Skipping permission test on Windows")
	}
}
