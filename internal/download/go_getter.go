package download

import "github.com/hashicorp/go-getter"

var getters = map[string]getter.Getter{
	"file":  &getter.FileGetter{Copy: true},
	"git":   new(getter.GitGetter),
	"https": &getter.HttpGetter{Netrc: true},
}

type GoGetterDownloader struct{}

func (g *GoGetterDownloader) Get(source string, destination string) error {
	return getter.GetAny(destination, source, getter.WithGetters(getters))
}

func NewGoGetterDownloader() Downloader {
	return &GoGetterDownloader{}
}
