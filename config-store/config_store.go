package config_store

import (
	"github.com/hashicorp/consul/api"
)

type ConfigStoreManager struct {
	client *api.Client
	prefix string
}

func NewConfigStoreManager(prefix string) (*ConfigStoreManager, error) {
	clientConfig := &api.Config{}

	configStoreManager := &api.Client{}

	return nil, nil
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

func (c *ConfigStoreManager) boshKeyToConsulPath(key string) string {
	return ""
}

func (c *ConfigStoreManager) consulPathToBOSHKey(key string) string {
	return ""
}
