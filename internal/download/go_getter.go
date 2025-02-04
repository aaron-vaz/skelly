package download

import (
	"context"

	"github.com/hashicorp/go-getter/v2"
)

type GoGetterDownloader struct{}

func (g *GoGetterDownloader) Get(source string, destination string) error {
	_, err := getter.GetAny(context.TODO(), destination, source)
	return err
}

func NewGoGetterDownloader() Downloader {
	return &GoGetterDownloader{}
}
