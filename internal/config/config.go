// Package config stores skelly's per-user settings, such as the template
// sources registered under an alias, in a YAML file on the user's machine.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	resolvePath func() (string, error)
}

// Load reads the config file, treating a missing file as an empty config.
func (s *FileStore) Load() (*Config, error) {
	path, err := s.resolvePath()
	if err != nil {
		// Without a config location nothing can have been saved, and commands
		// that merely consult the config should keep working
		return &Config{}, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config file, creating its directory if needed. The file is
// replaced atomically so an interrupted save cannot leave it half-written.
func (s *FileStore) Save(cfg *Config) error {
	path, err := s.resolvePath()
	if err != nil {
		return err
	}
	// Write through a symlinked config (e.g. one kept in a dotfiles repo)
	// instead of replacing the link with a regular file
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	dir := filepath.Dir(path)
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
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// ValidateAlias reports whether alias can be used to name a source. Aliases are
// plain words so they can never be mistaken for a URL or a path.
func ValidateAlias(alias string) error {
	if !IsAlias(alias) {
		return fmt.Errorf("invalid alias %q: use letters, digits, '-' and '_', starting with a letter or digit", alias)
	}
	return nil
}

// IsAlias reports whether s has the shape of an alias.
func IsAlias(s string) bool {
	return aliasPattern.MatchString(s)
}

// RedactSource hides credentials embedded in a source URL, such as a token in
// https://token@github.com/org/repo, so the source can be shown safely.
func RedactSource(source string) string {
	// go-getter sources may force a getter with a "git::" style prefix
	prefix, rest := "", source
	if i := strings.Index(source, "::"); i >= 0 {
		prefix, rest = source[:i+2], source[i+2:]
	}

	u, err := url.Parse(rest)
	if err != nil || u.User == nil {
		return source
	}
	u.User = url.User("REDACTED")
	return prefix + u.String()
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
	return &FileStore{resolvePath: func() (string, error) { return path, nil }}
}

// NewDefaultFileStore returns a Store that keeps the config at DefaultPath.
// The path is resolved on each use so commands that never touch the config
// still work when it cannot be located, e.g. with $HOME unset.
func NewDefaultFileStore() *FileStore {
	return &FileStore{resolvePath: DefaultPath}
}
