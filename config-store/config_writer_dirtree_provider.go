package configstore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hpcloud/fissile/model"
	"gopkg.in/yaml.v2"
)

const (
	leafFilename = "value.yml"
)

type dirTreeConfigWriterProvider struct {
	opinions   *opinions
	outputPath string
}

func newDirTreeConfigWriterProvider(opinions *opinions, allProps map[string]interface{}) (*dirTreeConfigWriterProvider, error) {
	return &dirTreeConfigWriterProvider{
		opinions: opinions,
	}, nil
}

func (d *dirTreeConfigWriterProvider) WriteConfigs(roleManifest *model.RoleManifest, builder *Builder) error {

	allReleases := make(map[*model.Release]bool)

	for _, role := range roleManifest.Roles {
		for _, job := range role.Jobs {
			allReleases[job.Release] = true
		}
	}

	d.outputPath = builder.targetLocation

	if err := os.RemoveAll(d.outputPath); err != nil && err != os.ErrNotExist {
		return err
	}

	for release := range allReleases {
		if err := d.writeDescriptionConfigs(release, builder); err != nil {
			return err
		}
	}

	for _, role := range roleManifest.Roles {
		for _, job := range role.Jobs {
			if err := d.writeSpecConfigs(job, builder); err != nil {
				return err
			}
		}
	}

	for release := range allReleases {
		if err := d.writeOpinionsConfigs(release, builder); err != nil {
			return err
		}
	}

	return nil
}

func (d *dirTreeConfigWriterProvider) writeSpecConfigs(job *model.Job, c *Builder) error {

	for _, property := range job.Properties {
		key, err := BoshKeyToConsulPath(fmt.Sprintf("%s.%s.%s", job.Release.Name, job.Name, property.Name), SpecStore)
		if err != nil {
			return err
		}

		if err := d.writeConfig(key, property.Default); err != nil {
			return err
		}
	}

	return nil
}

func (d *dirTreeConfigWriterProvider) writeDescriptionConfigs(release *model.Release, c *Builder) error {

	configs := release.GetUniqueConfigs()

	for _, config := range configs {
		key, err := BoshKeyToConsulPath(config.Name, DescriptionsStore)
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

		key, err := BoshKeyToConsulPath(config.Name, OpinionsStore)
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
	path := filepath.Join(d.outputPath, configKey)

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
