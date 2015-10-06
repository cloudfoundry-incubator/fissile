package config_store

import (
	"fmt"
	"strings"
)

type ConfigStoreManager struct {
	prefix string
}

func NewConfigStoreManager(prefix string) *ConfigStoreManager {
	configStoreManager := &ConfigStoreManager{
		prefix: prefix,
	}

	return configStoreManager
}

func (c *ConfigStoreManager) boshKeyToConsulPath(key string) (string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '.' })
	if len(keyGrams) == 0 {
		return "", fmt.Errorf("BOSH config key cannot be empty")
	}

	keyGrams = append([]string{"", c.prefix}, keyGrams...)
	return strings.Join(keyGrams, "/"), nil
}

func (c *ConfigStoreManager) consulPathToBOSHKey(key string) (string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '/' })
	if len(keyGrams) < 2 {
		return "", fmt.Errorf("Configuration key %s is not a valid consul key", key)
	}

	if keyGrams[0] != c.prefix {
		return "", fmt.Errorf("Consul configuration key %s does not have the correct prefix %s", key, c.prefix)
	}

	return strings.Join(keyGrams[1:], "."), nil
}
