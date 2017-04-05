package util

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
)

var (
	// DefaultLicensePrefixFilters for LoadLicenseFiles, NOTICE and LICENSE
	DefaultLicensePrefixFilters = []string{"LICENSE", "NOTICE"}
)

// LoadLicenseFiles iterates through a tar.gz file looking for anything that matches
// prefixFilters. Filename is for error generation only.
func LoadLicenseFiles(filename string, targz io.Reader, prefixFilters ...string) (map[string][]byte, error) {
	files := make(map[string][]byte)

	err := TargzIterate(filename, targz,
		func(licenseFile *tar.Reader, header *tar.Header) error {
			name := path.Base(header.Name)
			namePrefix := name[:len(name)-len(path.Ext(name))]
			found := false
			for _, filt := range prefixFilters {
				if namePrefix == filt {
					found = true
					break
				}
			}

			if !found {
				return nil
			}

			buf, err := ioutil.ReadAll(licenseFile)
			if err != nil {
				return err
			}

			files[header.Name] = buf
			return nil
		})

	return files, err
}

// TargzIterate iterates over the files it finds in a tar.gz file and calls a
// callback for each file encountered. Filename is only used for error generation.
func TargzIterate(filename string, targz io.Reader, fn func(*tar.Reader, *tar.Header) error) error {
	gzipReader, err := gzip.NewReader(targz)
	if err != nil {
		return fmt.Errorf("%s could not be read: %v", filename, err)
	}

	tarfile := tar.NewReader(gzipReader)
	for {
		header, err := tarfile.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("%s's tar'd files failed to read: %v", filename, err)
		}

		err = fn(tarfile, header)
		if err != nil {
			return err
		}
	}
}

// WriteHeaderToTarStream writes a tar header with default values as appropriate
func WriteHeaderToTarStream(stream *tar.Writer, header tar.Header) error {
	if header.Mode == 0 {
		header.Mode = 0644
	}
	if header.Typeflag == 0 {
		header.Typeflag = tar.TypeReg
	}
	if err := stream.WriteHeader(&header); err != nil {
		return err
	}
	return nil
}

// WriteToTarStream writes a byte array of data into a tar stream
func WriteToTarStream(stream *tar.Writer, data []byte, header tar.Header) error {
	if header.Size == 0 {
		header.Size = int64(len(data))
	}
	if err := WriteHeaderToTarStream(stream, header); err != nil {
		return err
	}
	if _, err := stream.Write(data); err != nil {
		return err
	}
	return nil
}

// CopyFileToTarStream writes a file to a tar stream
func CopyFileToTarStream(stream *tar.Writer, path string, header *tar.Header) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hdr := tar.Header(*header)
	if hdr.Size == 0 {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		hdr.Size = info.Size()
	}
	if err := WriteHeaderToTarStream(stream, hdr); err != nil {
		return err
	}
	if _, err := io.Copy(stream, file); err != nil {
		return err
	}
	return nil
}
