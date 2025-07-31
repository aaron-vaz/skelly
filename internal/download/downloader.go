package download

// Downloader provides functionality to download files from a source to a destination
type Downloader interface {
	// Get downloads a file from the given source to the specified destination path
	// Returns an error if the download fails
	Get(source string, destination string) error
}
