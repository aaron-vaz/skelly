package download

import (
	"context"

	"github.com/hashicorp/go-getter/v2"
)

type GoGetterDownloader struct{}

func (g *GoGetterDownloader) Get(ctx context.Context, source string, destination string) error {
	_, err := getter.GetAny(ctx, destination, source)
	return err
}

func NewGoGetterDownloader() Downloader {
	return &GoGetterDownloader{}
}
