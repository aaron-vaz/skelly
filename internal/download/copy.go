package download

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyDir recursively copies the directory tree at source into destination.
// Symlinks are recreated as links rather than followed, so a template can
// never pull in, or write through to, files outside its own tree.
func CopyDir(ctx context.Context, source string, destination string) error {
	// Resolve the root itself so a symlinked template directory still works
	root, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		switch {
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case d.Type().IsRegular():
			return copyFile(path, target, info.Mode().Perm())
		default:
			// Sockets, devices and pipes have no place in a project template
			return nil
		}
	})
}

func copyFile(source string, destination string, perm fs.FileMode) (err error) {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	// Close flushes the write, so its error must not be lost
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close %s: %w", destination, cerr)
		}
	}()

	_, err = io.Copy(out, in)
	return err
}
