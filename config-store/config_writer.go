package configstore

import ()

type ConfigWriter interface {
	WriteConfig(configKey string, value interface{}) error
	Save(targetPath string) error
}
