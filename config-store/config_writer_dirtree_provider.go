package configstore

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/termie/go-shutil"
	"gopkg.in/yaml.v2"
)

const (
	leafFilename = "file.yml"
)

type DirTreeConfigWriterProvider struct {
	tempDir string
}

func newDirTreeConfigWriterProvider() (*DirTreeConfigWriterProvider, error) {
	tempDir, err := ioutil.TempDir("", "fissile-config-writer-dirtree")
	if err != nil {
		return nil, err
	}

	return &DirTreeConfigWriterProvider{
		tempDir: tempDir,
	}, nil
}

func (d *DirTreeConfigWriterProvider) WriteConfig(configKey string, value interface{}) error {
	path := filepath.Join(d.tempDir, configKey)

	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(path, leafFilename)

	buf, err := yaml.Marshal(value)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filePath, buf, 0755)
}

func (d *DirTreeConfigWriterProvider) Save(targetPath string) error {
	return shutil.CopyTree(
		d.tempDir,
		targetPath,
		&shutil.CopyTreeOptions{
			Symlinks:               true,
			Ignore:                 nil,
			CopyFunction:           shutil.Copy,
			IgnoreDanglingSymlinks: false},
	)
}
