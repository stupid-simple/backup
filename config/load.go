package config

import (
	"encoding/json"
	"os"
)

func LoadFromFile(path string) (*Config, error) {
	cfgFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = cfgFile.Close()
	}()

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Config{}
	err = json.Unmarshal(raw, &cfg)
	return &cfg, err
}
