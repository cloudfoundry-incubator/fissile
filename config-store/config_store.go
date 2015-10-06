package config_store

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/consul/api"
)

type ConfigStoreManager struct {
	client *api.Client
	prefix string
}

func NewConfigStoreManager(prefix, consulURL string) (*ConfigStoreManager, error) {

	parsedURL, err := url.Parse(consulURL)
	if err != nil {
		return nil, err
	}

	consulScheme := parsedURL.Scheme
	consulAddress := parsedURL.Host
	consulDatacenter := strings.Trim(parsedURL.Path, "/")
	consulUsername := ""
	consulPassword := ""
	if parsedURL.User != nil {
		consulUsername = parsedURL.User.Username()
		consulPassword, _ = parsedURL.User.Password()
	}

	clientConfig := &api.Config{
		Address:    consulAddress,
		Scheme:     consulScheme,
		Datacenter: consulDatacenter,
		HttpAuth: &api.HttpBasicAuth{
			Username: consulUsername,
			Password: consulPassword,
		},
	}

	consulClient, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	configStoreManager := &ConfigStoreManager{
		client: consulClient,
		prefix: prefix,
	}

	return configStoreManager, nil
}

func (c *ConfigStoreManager) AddBOSHKey(key, description string, value interface{}) error {
	return nil
}

func (c *ConfigStoreManager) DeleteBOSHKey(key string) error {
	return nil
}

func (c *ConfigStoreManager) ListBOSHKeys() ([]string, error) {
	return nil, nil
}

func (c *ConfigStoreManager) GetBOSHKey(key string) (interface{}, error) {
	return nil, nil
}

func (c *ConfigStoreManager) boshKeyToConsulPath(key string) (string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '.' })
	if len(keyGrams) == 0 {
		return "", fmt.Errorf("BOSH config key cannot be empty", key)
	}

	keyGrams = append([]string{"", c.prefix}, keyGrams...)
	return strings.Join(keyGrams, "/")
}

func (c *ConfigStoreManager) consulPathToBOSHKey(key string) (string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '/' })
	if len(keyGrams) < 1 {
		return fmt.Errorf("Configuration key %s is not a valid consul key", key)
	}

	if keyGrams[0] != c.prefix {
		return fmt.Errorf("Consul configuration key %s does not have the correct prefix %s", key, c.prefix)
	}

	return strings.Join(keyGrams[1:], "."), nil
}
