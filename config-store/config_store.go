package configstore

import (
	"fmt"
	"strings"

	"github.com/hpcloud/fissile/model"
)

const (
	// SpecStore is the prefix for the spec defaults keys
	SpecStore = "spec"
	// OpinionsStore is the prefix for the opinions defaults keys
	OpinionsStore = "opinions"

	// DescriptionsStore is the prefix for the description keys
	DescriptionsStore = "descriptions"

	// DirTreeProvider is the name of the default config writer that creates a filesystem tree
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

// WriteBaseConfig generates the configuration base for a BOSH release
func (c *Builder) WriteBaseConfig(release *model.Release) error {
	var writer configWriter
	var err error

	switch {
	case c.provider == DirTreeProvider:
		writer, err = newDirTreeConfigWriterProvider()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid config writer provider %s", c.provider)
	}

	if err := c.writeDescriptionConfigs(release, writer); err != nil {
		return err
	}

	if err := c.writeSpecConfigs(release, writer); err != nil {
		return err
	}

	if err := c.writeOpinionsConfigs(release, writer); err != nil {
		return err
	}

	return writer.Save(c.targetLocation)
}

func (c *Builder) writeSpecConfigs(release *model.Release, confWriter configWriter) error {

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

func (c *Builder) writeDescriptionConfigs(release *model.Release, confWriter configWriter) error {

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

func (c *Builder) writeOpinionsConfigs(release *model.Release, confWriter configWriter) error {
	configs := release.GetUniqueConfigs()

	opinions, err := newOpinions(c.lightOpinionsPath, c.darkOpinionsPath)
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
