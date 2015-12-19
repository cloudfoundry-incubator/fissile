package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadLicenseFiles(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	licenseTar := filepath.Join(workDir, "../test-assets/tarReadTest.tar.gz")

	f, err := os.Open(licenseTar)
	assert.Nil(err)

	files, err := LoadLicenseFiles(licenseTar, f, DefaultLicensePrefixFilters...)
	assert.Nil(err)

	assert.Equal(1, len(files))
	assert.Equal(files["LICENSE"], []byte("license file\n"))
}
