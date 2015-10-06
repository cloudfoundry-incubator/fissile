package configstore

import (
	"fmt"
	"strings"

	"github.com/hpcloud/fissile/model"
)

// Builder creates a base configuration to be fed into Consul or someting similar
type Builder struct {
	prefix   string
	provider string
}

// NewConfigStoreBuilder creates a new configstore.Builder
func NewConfigStoreBuilder(prefix string) *Builder {
	configStoreManager := &Builder{
		prefix: prefix,
	}

	return configStoreManager
}

func (c *Builder) WriteBaseConfig(release *model.Release) error {
	return nil
}

func (c *Builder) writeSpecConfigs(release *model.Release) error {
	return nil
}

func (c *Builder) writeManifestConfigs() error {
	return nil
}

func (c *Builder) boshKeyToConsulPath(key string) (string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '.' })
	if len(keyGrams) == 0 {
		return "", fmt.Errorf("BOSH config key cannot be empty")
	}

	keyGrams = append([]string{"", c.prefix}, keyGrams...)
	return strings.Join(keyGrams, "/"), nil
}

func (c *Builder) consulPathToBOSHKey(key string) (string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '/' })
	if len(keyGrams) < 2 {
		return "", fmt.Errorf("Configuration key %s is not a valid consul key", key)
	}

	if keyGrams[0] != c.prefix {
		return "", fmt.Errorf("Consul configuration key %s does not have the correct prefix %s", key, c.prefix)
	}

	return strings.Join(keyGrams[1:], "."), nil
}
