package configstore

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/termie/go-shutil"
	"gopkg.in/yaml.v2"
)

const (
	leafFilename = "value.yml"
)

type dirTreeConfigWriterProvider struct {
	tempDir string
	prefix  string
}

func newDirTreeConfigWriterProvider(prefix string) (*dirTreeConfigWriterProvider, error) {
	tempDir, err := ioutil.TempDir("", "fissile-config-writer-dirtree")
	if err != nil {
		return nil, err
	}

	return &dirTreeConfigWriterProvider{
		tempDir: tempDir,
	}, nil
}

func (d *dirTreeConfigWriterProvider) WriteConfig(configKey string, value interface{}) error {
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

func (d *dirTreeConfigWriterProvider) Save(targetPath string) error {
	configDirSource := filepath.Join(d.tempDir, d.prefix)
	configDirDest := filepath.Join(targetPath, d.prefix)

	defer os.RemoveAll(d.tempDir)

	if err := os.RemoveAll(configDirDest); err != nil {
		return err
	}

	return shutil.CopyTree(
		configDirSource,
		configDirDest,
		&shutil.CopyTreeOptions{
			Symlinks:               true,
			Ignore:                 nil,
			CopyFunction:           shutil.Copy,
			IgnoreDanglingSymlinks: false},
	)
}
