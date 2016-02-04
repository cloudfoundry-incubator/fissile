package configstore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hpcloud/fissile/model"
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

func (d *dirTreeConfigWriterProvider) WriteConfigsFromRelease(release *model.Release, c *Builder) error {
	if err := d.writeDescriptionConfigs(release, c); err != nil {
		return err
	}
	if err := d.writeSpecConfigs(release, c); err != nil {
		return err
	}
	if err := d.writeOpinionsConfigs(release, c); err != nil {
		return err
	}

	return nil
}

func (d *dirTreeConfigWriterProvider) writeSpecConfigs(release *model.Release, c *Builder) error {

	for _, job := range release.Jobs {
		for _, property := range job.Properties {
			key, err := c.boshKeyToConsulPath(fmt.Sprintf("%s.%s.%s", release.Name, job.Name, property.Name), SpecStore)
			if err != nil {
				return err
			}

			if err := d.writeConfig(key, property.Default); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *dirTreeConfigWriterProvider) writeDescriptionConfigs(release *model.Release, c *Builder) error {

	configs := release.GetUniqueConfigs()

	for _, config := range configs {
		key, err := c.boshKeyToConsulPath(config.Name, DescriptionsStore)
		if err != nil {
			return err
		}

		if err := d.writeConfig(key, config.Description); err != nil {
			return err
		}
	}

	return nil
}

func (d *dirTreeConfigWriterProvider) writeOpinionsConfigs(release *model.Release, c *Builder) error {
	configs := release.GetUniqueConfigs()

	opinions, err := newOpinions(c.lightOpinionsPath, c.darkOpinionsPath)
	if err != nil {
		return err
	}

	for _, config := range configs {
		keyPieces, err := getKeyGrams(config.Name)
		if err != nil {
			return err
		}

		masked, value := opinions.GetOpinionForKey(keyPieces)
		if masked || value == nil {
			continue
		}

		key, err := c.boshKeyToConsulPath(config.Name, OpinionsStore)
		if err != nil {
			return err
		}

		if err := d.writeConfig(key, value); err != nil {
			return err
		}
	}

	return nil
}

func (d *dirTreeConfigWriterProvider) writeConfig(configKey string, value interface{}) error {
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

func (d *dirTreeConfigWriterProvider) CleanUp() error {
	if err := os.RemoveAll(d.tempDir); err != nil && err != os.ErrNotExist {
		return err
	}
	return nil
}
