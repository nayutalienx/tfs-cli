package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	envBaseURL = "TFS_BASE_URL"
	envProject = "TFS_PROJECT"
	envPAT     = "TFS_PAT"
)

type Config struct {
	BaseURL string `json:"baseUrl"`
	Project string `json:"project"`
	PAT     string `json:"pat"`
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tfs", "config.json"), nil
}

func Load(path string) (Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func FromEnv() Config {
	return Config{
		BaseURL: os.Getenv(envBaseURL),
		Project: os.Getenv(envProject),
		PAT:     os.Getenv(envPAT),
	}
}

func Merge(base Config, override Config) Config {
	if override.BaseURL != "" {
		base.BaseURL = override.BaseURL
	}
	if override.Project != "" {
		base.Project = override.Project
	}
	if override.PAT != "" {
		base.PAT = override.PAT
	}
	return base
}

func (c Config) Redacted() Config {
	if c.PAT == "" {
		return c
	}
	return Config{BaseURL: c.BaseURL, Project: c.Project, PAT: "***"}
}

