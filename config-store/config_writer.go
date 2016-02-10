package configstore

import "github.com/hpcloud/fissile/model"

type configWriter interface {
	// WriteConfigs writes the configuration used in a role manifest to a Builder's targetLocation
	WriteConfigs(roleManifest *model.RoleManifest, c *Builder) error
}
