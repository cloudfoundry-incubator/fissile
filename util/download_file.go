package util

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/machinebox/progress"
)

type progressDelegate func(int)

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(filepath string, url string, progressEvent progressDelegate) error {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	transport := &http.Transport{}
	transport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	httpClient := &http.Client{Transport: transport}
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	size := resp.ContentLength
	reader := progress.NewReader(resp.Body)

	go func() {
		ctx := context.Background()
		progressChan := progress.NewTicker(ctx, reader, size, 1*time.Second)

		for p := range progressChan {
			progressEvent(int(p.Percent()))
		}
	}()

	// Write the body to file
	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}

	return nil
}
