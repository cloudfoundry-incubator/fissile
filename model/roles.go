package model

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	Roles []*Role `yaml:"roles"`
}

// Role represents a collection of jobs that are colocated on a container
type Role struct {
	Name string `yaml:"name"`
	Jobs []*Job `yaml:"_,omitempty"`

	JobNameList []*roleJob `yaml:"jobs"`
}

type roleJob struct {
	Name string `yaml:"name"`
}

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, release *Release) (*RoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	rolesManifest := RoleManifest{}
	if err := yaml.Unmarshal(manifestContents, &rolesManifest); err != nil {
		return nil, err
	}

	for _, role := range rolesManifest.Roles {
		role.Jobs = make([]*Job, len(role.JobNameList))

		for idx, roleJob := range role.JobNameList {
			job, err := release.LookupJob(roleJob.Name)
			if err != nil {
				return nil, err
			}

			role.Jobs[idx] = job
		}
	}

	return &rolesManifest, nil
}
