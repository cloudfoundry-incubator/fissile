package model

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	Roles []*Role `yaml:"roles"`

	manifestFilePath string
}

// Role represents a collection of jobs that are colocated on a container
type Role struct {
	Name        string     `yaml:"name"`
	Jobs        []*Job     `yaml:"_,omitempty"`
	Scripts     []string   `yaml:"scripts"`
	IsTask      bool       `yaml:"is_task,omitempty"`
	JobNameList []*roleJob `yaml:"jobs"`

	rolesManifest *RoleManifest
}

type roleJob struct {
	Name        string `yaml:"name"`
	ReleaseName string `yaml:"release_name"`
}

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, releases []*Release) (*RoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	mappedReleases := map[string]*Release{}

	for _, release := range releases {
		_, ok := mappedReleases[release.Name]

		if ok {
			return nil, fmt.Errorf("Error - release %s has been loaded more than once.", release.Name)
		}

		mappedReleases[release.Name] = release
	}

	rolesManifest := RoleManifest{}
	rolesManifest.manifestFilePath = manifestFilePath
	if err := yaml.Unmarshal(manifestContents, &rolesManifest); err != nil {
		return nil, err
	}

	for _, role := range rolesManifest.Roles {
		role.rolesManifest = &rolesManifest
		role.Jobs = make([]*Job, len(role.JobNameList))

		for idx, roleJob := range role.JobNameList {
			release, ok := mappedReleases[roleJob.ReleaseName]

			if !ok {
				return nil, fmt.Errorf("Error - release %s has not been loaded and is referenced by job %s in role %s.",
					roleJob.ReleaseName, roleJob.Name, role.Name)
			}

			job, err := release.LookupJob(roleJob.Name)
			if err != nil {
				return nil, err
			}

			role.Jobs[idx] = job
		}
	}

	return &rolesManifest, nil
}

// GetScriptPaths returns the paths to the startup scripts for a role
func (r *Role) GetScriptPaths() map[string]string {
	result := map[string]string{}

	if r.Scripts == nil {
		return result
	}

	for _, script := range r.Scripts {
		result[script] = filepath.Join(filepath.Dir(r.rolesManifest.manifestFilePath), script)
	}

	return result

}
