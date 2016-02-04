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

	// JSONProvider is the name of the JSON output provider; outputs one file per job
	JSONProvider = "json"
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
func (c *Builder) WriteBaseConfig(releases []*model.Release) error {
	var writer configWriter
	var err error

	switch {
	case c.provider == DirTreeProvider:
		writer, err = newDirTreeConfigWriterProvider(c.prefix)
		if err != nil {
			return err
		}
	case c.provider == JSONProvider:
		writer, err = newJSONConfigWriterProvider(c.prefix)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid config writer provider %s", c.provider)
	}
	defer writer.CleanUp()

	for _, release := range releases {
		if err := writer.WriteConfigsFromRelease(release, c); err != nil {
			fmt.Printf("Error writing configs for %s: %+v\n", release.Name, err)
			return err
		}
	}

	return writer.Save(c.targetLocation)
}

func (c *Builder) boshKeyToConsulPath(key, store string) (string, error) {
	keyGrams, err := getKeyGrams(key)
	if err != nil {
		return "", err
	}

	keyGrams = append([]string{"", c.prefix, store}, keyGrams...)
	return strings.Join(keyGrams, "/"), nil
}
