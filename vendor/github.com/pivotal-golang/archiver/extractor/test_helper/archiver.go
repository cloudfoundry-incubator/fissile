package test_helper

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"

	. "github.com/onsi/gomega"
)

type ArchiveFile struct {
	Name string
	Body string
	Mode int64
	Dir  bool
	Link string
}

func CreateZipArchive(filename string, files []ArchiveFile) {
	file, err := os.Create(filename)
	Ω(err).ShouldNot(HaveOccurred())

	w := zip.NewWriter(file)

	for _, file := range files {
		header := &zip.FileHeader{
			Name: file.Name,
		}

		mode := file.Mode
		if mode == 0 {
			mode = 0777
		}

		if file.Link != "" {
			header.SetMode(os.FileMode(mode) | os.ModeSymlink)
		} else {
			header.SetMode(os.FileMode(mode))
		}

		f, err := w.CreateHeader(header)
		Ω(err).ShouldNot(HaveOccurred())

		if file.Link != "" {
			_, err = f.Write([]byte(file.Link))
		} else {
			_, err = f.Write([]byte(file.Body))
		}
		Ω(err).ShouldNot(HaveOccurred())
	}

	err = w.Close()
	Ω(err).ShouldNot(HaveOccurred())

	err = file.Close()
	Ω(err).ShouldNot(HaveOccurred())
}

func CreateTarGZArchive(filename string, files []ArchiveFile) {
	file, err := os.Create(filename)
	Ω(err).ShouldNot(HaveOccurred())

	gw := gzip.NewWriter(file)

	WriteTar(gw, files)

	err = gw.Close()
	Ω(err).ShouldNot(HaveOccurred())

	err = file.Close()
	Ω(err).ShouldNot(HaveOccurred())
}

func CreateTarArchive(filename string, files []ArchiveFile) {
	file, err := os.Create(filename)
	Ω(err).ShouldNot(HaveOccurred())

	WriteTar(file, files)

	err = file.Close()
	Ω(err).ShouldNot(HaveOccurred())
}

func WriteTar(destination io.Writer, files []ArchiveFile) {
	w := tar.NewWriter(destination)

	for _, file := range files {
		var header *tar.Header

		mode := file.Mode
		if mode == 0 {
			mode = 0777
		}

		if file.Dir {
			header = &tar.Header{
				Name:     file.Name,
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}
		} else if file.Link != "" {
			header = &tar.Header{
				Name:     file.Name,
				Typeflag: tar.TypeSymlink,
				Linkname: file.Link,
				Mode:     file.Mode,
			}
		} else {
			header = &tar.Header{
				Name: file.Name,
				Mode: mode,
				Size: int64(len(file.Body)),
			}
		}

		err := w.WriteHeader(header)
		Ω(err).ShouldNot(HaveOccurred())

		_, err = w.Write([]byte(file.Body))
		Ω(err).ShouldNot(HaveOccurred())
	}

	err := w.Close()
	Ω(err).ShouldNot(HaveOccurred())
}
