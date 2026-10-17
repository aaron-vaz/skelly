package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStore_Load_MissingFile(t *testing.T) {
	// Given
	store := NewFileStore(filepath.Join(t.TempDir(), "missing", "config.yml"))

	// When
	cfg, err := store.Load()

	// Then
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	// And
	if len(cfg.Sources) != 0 {
		t.Errorf("Expected an empty config, got %v", cfg)
	}
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "nested", "config.yml")
	store := NewFileStore(path)
	want := &Config{Sources: map[string]Source{"go": {URL: "github.com/user/go-template"}}}

	// When
	err := store.Save(want)

	// Then
	if err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	// And
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Expected config file to exist: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file permissions = %o, want 600", perm)
	}

	// And
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if got.Sources["go"].URL != "github.com/user/go-template" || len(got.Sources) != 1 {
		t.Errorf("Load() = %v, want %v", got, want)
	}

	// And no temporary files are left behind
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("Expected only the config file, found %v", entries)
	}
}

func TestFileStore_Load_InvalidYAML(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("sources: [not a map"), 0o600); err != nil {
		t.Fatal(err)
	}

	// When
	_, err := NewFileStore(path).Load()

	// Then
	if err == nil || !strings.Contains(err.Error(), "failed to parse config") {
		t.Errorf("Load() error = %v, want parse error", err)
	}
}

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		alias string
		valid bool
	}{
		{"go", true},
		{"go-service_2", true},
		{"9lives", true},
		{"", false},
		{"-go", false},
		{"github.com", false},
		{"user/repo", false},
		{"./go", false},
		{"my alias", false},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			// When
			err := ValidateAlias(tt.alias)

			// Then
			if (err == nil) != tt.valid {
				t.Errorf("ValidateAlias(%q) error = %v, want valid %v", tt.alias, err, tt.valid)
			}
		})
	}
}

func TestDefaultPath(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "explicit override",
			env:  map[string]string{PathEnv: "/custom/skelly.yml", "XDG_CONFIG_HOME": "/xdg"},
			want: "/custom/skelly.yml",
		},
		{
			name: "xdg config home",
			env:  map[string]string{PathEnv: "", "XDG_CONFIG_HOME": "/xdg"},
			want: "/xdg/skelly/config.yml",
		},
		{
			name: "relative xdg config home is ignored",
			env:  map[string]string{PathEnv: "", "XDG_CONFIG_HOME": "xdg", "HOME": "/home/jane"},
			want: "/home/jane/.config/skelly/config.yml",
		},
		{
			name: "home directory fallback",
			env:  map[string]string{PathEnv: "", "XDG_CONFIG_HOME": "", "HOME": "/home/jane"},
			want: "/home/jane/.config/skelly/config.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			// When
			got, err := DefaultPath()

			// Then
			if err != nil {
				t.Fatalf("DefaultPath() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("DefaultPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileStore_Save_ThroughSymlink(t *testing.T) {
	// Given a config file that is a symlink, e.g. into a dotfiles repo
	target := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(target, []byte("sources: {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "config.yml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	// When
	err := NewFileStore(link).Save(&Config{Sources: map[string]Source{"go": {URL: "github.com/user/go"}}})

	// Then
	if err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected config to remain a symlink, got %v (%v)", info, err)
	}

	// And the linked file holds the new config
	data, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(data), "github.com/user/go") {
		t.Errorf("Expected target to be updated, got %q (%v)", data, err)
	}
}

func TestDefaultFileStore_NoConfigLocation(t *testing.T) {
	// Given no way to locate the config file
	t.Setenv(PathEnv, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	store := NewDefaultFileStore()

	// When
	cfg, err := store.Load()

	// Then loading still works so commands that only consult the config run
	if err != nil || len(cfg.Sources) != 0 {
		t.Errorf("Load() = %v, %v, want an empty config", cfg, err)
	}

	// And saving reports why it cannot
	if err := store.Save(&Config{}); err == nil {
		t.Error("Save() error = nil, want failure to locate config file")
	}
}

func TestRedactSource(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"github.com/user/tpl", "github.com/user/tpl"},
		{"/home/u/templates/go", "/home/u/templates/go"},
		{"git@github.com:org/tpl.git", "git@github.com:org/tpl.git"},
		{"https://github.com/org/tpl.git", "https://github.com/org/tpl.git"},
		{"https://ghp_secret@github.com/org/tpl.git", "https://REDACTED@github.com/org/tpl.git"},
		{"https://user:pass@example.com/tpl.zip", "https://REDACTED@example.com/tpl.zip"},
		{"git::https://user:token@github.com/org/tpl.git?ref=v1", "git::https://REDACTED@github.com/org/tpl.git?ref=v1"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			// When
			got := RedactSource(tt.source)

			// Then
			if got != tt.want {
				t.Errorf("RedactSource(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}
