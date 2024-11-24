package download

type Downloader interface {
	Get(source string, destination string) error
}
