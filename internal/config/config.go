// Package config stores skelly's per-user settings, such as the template
// sources registered under an alias, in a YAML file on the user's machine.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const (
	// PathEnv overrides the location of the config file.
	PathEnv  = "SKELLY_CONFIG"
	fileName = "config.yml"
	dirName  = "skelly"
)

var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Store loads and saves the user's config.
type Store interface {
	// Load returns the stored config, or an empty one if none has been saved yet
	Load() (*Config, error)
	// Save replaces the stored config with cfg
	Save(cfg *Config) error
}

// Source is a template source registered under an alias.
type Source struct {
	URL string `yaml:"url"`
}

// Config holds every user setting.
type Config struct {
	Sources map[string]Source `yaml:"sources,omitempty"`
}

// FileStore is a Store backed by a YAML file.
type FileStore struct {
	path string
}

// Load reads the config file, treating a missing file as an empty config.
func (s *FileStore) Load() (*Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", s.path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", s.path, err)
	}
	return cfg, nil
}

// Save writes the config file, creating its directory if needed. The file is
// replaced atomically so an interrupted save cannot leave it half-written.
func (s *FileStore) Save(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Sources may embed credentials, e.g. a token in a git URL, so keep the file private
	tmp, err := os.CreateTemp(dir, "."+fileName+"-*")
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// ValidateAlias reports whether alias can be used to name a source. Aliases are
// plain words so they can never be mistaken for a URL or a path.
func ValidateAlias(alias string) error {
	if !aliasPattern.MatchString(alias) {
		return fmt.Errorf("invalid alias %q: use letters, digits, '-' and '_', starting with a letter or digit", alias)
	}
	return nil
}

// IsAlias reports whether s has the shape of an alias.
func IsAlias(s string) bool {
	return aliasPattern.MatchString(s)
}

// DefaultPath returns $SKELLY_CONFIG if set, otherwise config.yml under
// $XDG_CONFIG_HOME/skelly, falling back to ~/.config/skelly.
func DefaultPath() (string, error) {
	if p := os.Getenv(PathEnv); p != "" {
		return p, nil
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, dirName, fileName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to locate config file: %w", err)
	}
	return filepath.Join(home, ".config", dirName, fileName), nil
}

// NewFileStore returns a Store that keeps the config in the file at path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}
