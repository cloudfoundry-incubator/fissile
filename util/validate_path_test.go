package util

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePath(t *testing.T) {
	assert := assert.New(t)

	baseDir, err := ioutil.TempDir("", "fissile-")
	assert.NoError(err)
	defer os.RemoveAll(baseDir)

	checkDir, err := ioutil.TempDir(baseDir, "checkdir-")
	assert.NoError(err)

	checkFileName := path.Join(baseDir, "checkfile")
	checkFile, err := os.Create(checkFileName)
	assert.NoError(err)

	err = checkFile.Close()
	assert.NoError(err)

	// Check for missing file
	err = ValidatePath(path.Join(baseDir, "foo"), false, "missing file")
	assert.Error(err)
	assert.Contains(err.Error(), "missing file")

	// Check for dir when it should be a file
	err = ValidatePath(checkDir, false, "should be file")
	assert.Error(err)
	assert.Contains(err.Error(), "should be file")

	// Check for file when it should be a dir
	err = ValidatePath(checkFileName, true, "should be dir")
	assert.Error(err)
	assert.Contains(err.Error(), "should be dir")
}
