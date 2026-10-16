package download

import (
	"context"
)

// LocalDownloader implements Downloader for local directory copying.
// It bypasses remote fetching logic to perform direct, physical
// filesystem copies, avoiding go-getter's symbolic linking of local sources.
type LocalDownloader struct{}

// Get executes a recursive directory copy from the specified source
// to the designated destination, preserving the local file hierarchy.
func (d *LocalDownloader) Get(ctx context.Context, source string, destination string) error {
	return CopyDir(ctx, source, destination)
}

// NewLocalDownloader initializes and returns a robust Downloader explicitly
// designed to safely traverse and copy local template directories.
func NewLocalDownloader() Downloader {
	return &LocalDownloader{}
}
