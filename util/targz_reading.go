package util

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
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
