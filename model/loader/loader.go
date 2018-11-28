package loader

// split the loader into a separate package. in case we have several release
// resolvers we want to keep `dep ensure` small

import (
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/releaseresolver"
	"code.cloudfoundry.org/fissile/model/resolver"
)

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, options model.LoadRoleManifestOptions) (*model.RoleManifest, error) {
	roleManifest := model.NewRoleManifest()
	err := roleManifest.LoadManifestFromFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	r := releaseresolver.NewReleaseResolver(manifestFilePath)
	return resolver.NewResolver(roleManifest, r, options).Resolve()
}
