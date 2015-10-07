package configstore

import (
	"fmt"
	"strings"

	"github.com/hpcloud/fissile/model"
)

const (
	SpecStore         = "spec"
	OpinionsStore     = "opinions"
	DescriptionsStore = "descriptions"

	DirTreeProvider = "dirtree"
)

// Builder creates a base configuration to be fed into Consul or something similar
type Builder struct {
	prefix            string
	provider          string
	lightOpinionsPath string
	darkOpinionsPath  string
	targetLocation    string
}

// NewConfigStoreBuilder creates a new configstore.Builder
func NewConfigStoreBuilder(prefix, provider, lightOpinionsPath, darkOpinionsPath, targetLocation string) *Builder {
	configStoreManager := &Builder{
		prefix:            prefix,
		provider:          provider,
		lightOpinionsPath: lightOpinionsPath,
		darkOpinionsPath:  darkOpinionsPath,
		targetLocation:    targetLocation,
	}

	return configStoreManager
}

func (c *Builder) WriteBaseConfig(release *model.Release) error {
	var configWriter ConfigWriter
	var err error

	switch {
	case c.provider == DirTreeProvider:
		configWriter, err = newDirTreeConfigWriterProvider()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid config writer provider %s", c.provider)
	}

	if err := c.writeDescriptionConfigs(release, configWriter); err != nil {
		return err
	}

	if err := c.writeSpecConfigs(release, configWriter); err != nil {
		return err
	}

	if err := c.writeOpinionsConfigs(release, configWriter); err != nil {
		return err
	}

	return configWriter.Save(c.targetLocation)
}

func (c *Builder) writeSpecConfigs(release *model.Release, confWriter ConfigWriter) error {

	for _, job := range release.Jobs {
		for _, property := range job.Properties {
			key, err := c.boshKeyToConsulPath(fmt.Sprintf("%s.%s", job.Name, property.Name), SpecStore)
			if err != nil {
				return err
			}

			if err := confWriter.WriteConfig(key, property.Default); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Builder) writeDescriptionConfigs(release *model.Release, confWriter ConfigWriter) error {

	configs := release.GetUniqueConfigs()

	for _, config := range configs {
		key, err := c.boshKeyToConsulPath(config.Name, DescriptionsStore)
		if err != nil {
			return err
		}

		if err := confWriter.WriteConfig(key, config.Description); err != nil {
			return err
		}
	}

	return nil
}

func (c *Builder) writeOpinionsConfigs(release *model.Release, confWriter ConfigWriter) error {
	configs := release.GetUniqueConfigs()

	opinions, err := NewOpinions(c.lightOpinionsPath, c.darkOpinionsPath)
	if err != nil {
		return err
	}

	for _, config := range configs {
		keyPieces, err := c.getKeyGrams(config.Name)
		if err != nil {
			return err
		}

		value := opinions.GetOpinionForKey(keyPieces)
		if value == nil {
			continue
		}

		key, err := c.boshKeyToConsulPath(config.Name, OpinionsStore)
		if err != nil {
			return err
		}

		if err := confWriter.WriteConfig(key, value); err != nil {
			return err
		}
	}

	return nil
}

func (c *Builder) getKeyGrams(key string) ([]string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '.' })
	if len(keyGrams) == 0 {
		return nil, fmt.Errorf("BOSH config key cannot be empty")
	}

	return keyGrams, nil
}

func (c *Builder) boshKeyToConsulPath(key, store string) (string, error) {
	keyGrams, err := c.getKeyGrams(key)
	if err != nil {
		return "", err
	}

	keyGrams = append([]string{"", c.prefix, store}, keyGrams...)
	return strings.Join(keyGrams, "/"), nil
}
