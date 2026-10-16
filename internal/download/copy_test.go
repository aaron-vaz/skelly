package download

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir_PreservesSymlinks(t *testing.T) {
	// Given
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(outside, []byte("secret"), 0644)
	_ = os.MkdirAll(filepath.Join(src, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(src, "sub", "file.txt"), []byte("hello"), 0600)
	if err := os.Symlink(outside, filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}

	// When
	err := CopyDir(context.Background(), src, dest)

	// Then
	if err != nil {
		t.Fatalf("CopyDir() error = %v", err)
	}

	// And
	target, err := os.Readlink(filepath.Join(dest, "link"))
	if err != nil || target != outside {
		t.Errorf("Expected link to be recreated pointing at %s, got %q (%v)", outside, target, err)
	}

	// And
	info, err := os.Stat(filepath.Join(dest, "sub", "file.txt"))
	if err != nil {
		t.Fatalf("Expected nested file to be copied: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600 to be preserved, got %v", info.Mode().Perm())
	}
}

func TestCopyDir_SymlinkedRoot(t *testing.T) {
	// Given
	realDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(realDir, "a.txt"), []byte("a"), 0644)
	link := filepath.Join(t.TempDir(), "tpl")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "dest")

	// When
	err := CopyDir(context.Background(), link, dest)

	// Then
	if err != nil {
		t.Fatalf("CopyDir() error = %v", err)
	}

	// And
	if _, err := os.Stat(filepath.Join(dest, "a.txt")); err != nil {
		t.Errorf("Expected contents of symlinked root to be copied: %v", err)
	}
}

func TestCopyDir_Cancelled(t *testing.T) {
	// Given
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	err := CopyDir(ctx, src, filepath.Join(t.TempDir(), "dest"))

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Errorf("CopyDir() error = %v, want context.Canceled", err)
	}
}
