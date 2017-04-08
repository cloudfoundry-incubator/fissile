package util

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadLicenseFiles(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	licenseTar := filepath.Join(workDir, "../test-assets/tarReadTest.tar.gz")

	f, err := os.Open(licenseTar)
	assert.NoError(err)

	files, err := LoadLicenseFiles(licenseTar, f, DefaultLicensePrefixFilters...)
	assert.NoError(err)

	assert.Len(files, 1)
	assert.Equal(files["LICENSE"], []byte("license file\n"))
}

func TestWriteToTarStream(t *testing.T) {
	assert := assert.New(t)

	buf := bytes.Buffer{}
	expected := []byte("hello")

	writer := tar.NewWriter(&buf)
	err := WriteToTarStream(writer, expected, tar.Header{Name: "hello.txt"})
	assert.NoError(err)
	assert.NoError(writer.Close())

	reader := tar.NewReader(&buf)
	header, err := reader.Next()
	assert.NoError(err)

	assert.Equal(header.Name, "hello.txt")
	assert.EqualValues(0644, header.Mode, "Did not get default file mode")
	assert.EqualValues(tar.TypeReg, header.Typeflag, "Did not get default file type")

	actual, err := ioutil.ReadAll(reader)
	assert.NoError(err)
	assert.Equal(expected, actual, "Incorrect data read")
}

func TestCopyFileToTarStream(t *testing.T) {
	assert := assert.New(t)

	buf := bytes.Buffer{}
	expected := []byte("hello")

	tempFile, err := ioutil.TempFile("", "fissile-tar-test")
	assert.NoError(err)
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	_, err = tempFile.Write(expected)
	assert.NoError(err)
	assert.NoError(tempFile.Close())

	writer := tar.NewWriter(&buf)
	err = CopyFileToTarStream(writer, tempPath, &tar.Header{Name: "hello.txt"})
	assert.NoError(err)
	assert.NoError(writer.Close())

	reader := tar.NewReader(&buf)
	header, err := reader.Next()
	assert.NoError(err)

	assert.Equal(header.Name, "hello.txt")
	assert.EqualValues(0644, header.Mode, "Did not get default file mode")
	assert.EqualValues(tar.TypeReg, header.Typeflag, "Did not get default file type")
	assert.EqualValues(len(expected), header.Size, "Incorrect file size")

	actual, err := ioutil.ReadAll(reader)
	assert.NoError(err)
	assert.Equal(expected, actual, "Incorrect data read")
}
