package util

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestDownloadFile(test *testing.T) {

	// Prevent to go in endless loop if test fails
	var testTimeout = time.Duration(60)
	var testData = "Fissile test data"
	var testRoute = "/download"
	var testFile = "test_download"
	var testHost = "127.0.0.1"

	dir, err := ioutil.TempDir("", testFile)
	if err != nil {
		test.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tempFile := path.Join(dir, testFile)

	router := http.NewServeMux()
	router.Handle(testRoute, servetestdata([]byte(testData)))

	// Create a simple http server so the test can run also when network is disabled
	server := &http.Server{Handler: router}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	host := testHost + ":" + strconv.Itoa(port)

	done := make(chan bool)
	quit := make(chan bool)

	go func() {
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		server.Shutdown(ctx)
		close(done)
	}()

	// Quit the server after timeout exhausted
	go func() {
		timer := time.NewTimer(testTimeout * time.Second)
		<-timer.C
		quit <- true
	}()
	u, err := url.Parse("http://" + host)
	if err != nil {
		test.Fatal(err)
	}
	u.Path = path.Join(u.Path, testRoute)

	go server.Serve(listener)

	downloaderr := DownloadFile(tempFile, u.String(), func(i int) {})
	if downloaderr != nil {
		test.Fatal(downloaderr)
	}
	quit <- true
	<-done

	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		test.Fatal("File was not downloaded")
	}

	b, err := ioutil.ReadFile(tempFile)
	if err != nil {
		test.Fatal(err)
	}

	str := string(b)
	if str != testData {
		test.Fatal("File corrupted: " + str)
	}
}

func servetestdata(b []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(b)
	})
}
