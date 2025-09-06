package download

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-getter/v2"
)

type GoGetterDownloader struct{}

func (d *GoGetterDownloader) Get(ctx context.Context, source string, destination string) error {
	// For local sources, go-getter expects an absolute path.
	// We check if the source is a local path, and if so, convert it to an absolute path.
	if _, err := os.Stat(source); err == nil {
		absSrc, err := filepath.Abs(source)
		if err != nil {
			return err // Could not get absolute path
		}
		source = absSrc
	}

	_, err := getter.GetAny(ctx, destination, source)
	return err
}

func NewGoGetterDownloader() Downloader {
	return &GoGetterDownloader{}
}
