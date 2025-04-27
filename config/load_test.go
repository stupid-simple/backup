package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stupid-simple/backup/config"
)

var goodConfig = `
{
	"sources": [
		{
			"source_dir": "test1",
			"archive_dir": "test2",
			"enable": true,
			"cron": "* * * * *"
		},
		{
			"source_dir": "test3",
			"archive_dir": "test4",
			"enable": false,
			"cron": "10 * * * *"
		}
	]
}
`

var badConfig = `
[]
`

func TestLoad_Good(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "test.json")
	err := os.WriteFile(testFile, []byte(goodConfig), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFromFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(cfg.Sources))
	}

	if cfg.Sources[0].SourceDir != "test1" {
		t.Errorf("expected source test1, got %s", cfg.Sources[0].SourceDir)
	}

	if cfg.Sources[0].ArchiveDir != "test2" {
		t.Errorf("expected dest test1, got %s", cfg.Sources[0].ArchiveDir)
	}

	if cfg.Sources[1].SourceDir != "test3" {
		t.Errorf("expected source test2, got %s", cfg.Sources[1].SourceDir)
	}

	if cfg.Sources[1].ArchiveDir != "test4" {
		t.Errorf("expected dest test2, got %s", cfg.Sources[1].ArchiveDir)
	}
}

func TestLoad_Bad(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "test.json")
	err := os.WriteFile(testFile, []byte(badConfig), 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = config.LoadFromFile(testFile)
	if err == nil {
		t.Error("expected error")
	}
}

func TestLoad_NoFile(t *testing.T) {
	_, err := config.LoadFromFile("unexisting")
	if err == nil {
		t.Error("expected error")
	}
}

func TestLoad_Unreadable(t *testing.T) {
	_, err := config.LoadFromFile(t.TempDir())
	if err == nil {
		t.Error("expected error")
	}
}
