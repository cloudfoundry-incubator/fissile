package configstore

import "github.com/hpcloud/fissile/model"

type configWriter interface {
	WriteConfigs(role *model.Role, job *model.Job, c *Builder) error
	Save(targetPath string) error
	CleanUp() error
}
