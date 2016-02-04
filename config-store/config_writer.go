package configstore

import "github.com/hpcloud/fissile/model"

type configWriter interface {
	WriteConfigsFromRelease(release *model.Release, c *Builder) error
	Save(targetPath string) error
	CleanUp() error
}
