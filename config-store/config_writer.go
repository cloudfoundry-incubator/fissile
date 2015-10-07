package configstore

type configWriter interface {
	WriteConfig(configKey string, value interface{}) error
	Save(targetPath string) error
}
