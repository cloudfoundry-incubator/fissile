package model

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v2"
)

const (
	boshTaskType = "bosh-task"
	boshType     = "bosh"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	Roles         []*Role        `yaml:"roles"`
	Configuration *configuration `yaml:"configuration"`

	manifestFilePath string
}

// Role represents a collection of jobs that are colocated on a container
type Role struct {
	Name              string         `yaml:"name"`
	Jobs              Jobs           `yaml:"_,omitempty"`
	EnvironScripts    []string       `yaml:"environment_scripts"`
	Scripts           []string       `yaml:"scripts"`
	PostConfigScripts []string       `yaml:"post_config_scripts"`
	Type              string         `yaml:"type,omitempty"`
	JobNameList       []*roleJob     `yaml:"jobs"`
	Configuration     *configuration `yaml:"configuration"`

	rolesManifest *RoleManifest
}

type configuration struct {
	Templates map[string]string `yaml:"templates"`
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

	// Remove all roles that are not of the "bosh" or "bosh-task" type
	// Default type is considered to be "bosh"
	for i := len(rolesManifest.Roles) - 1; i >= 0; i-- {
		role := rolesManifest.Roles[i]

		if role.Type != "" && role.Type != boshTaskType && role.Type != boshType {
			rolesManifest.Roles = append(rolesManifest.Roles[:i], rolesManifest.Roles[i+1:]...)
		}
	}

	if rolesManifest.Configuration == nil {
		rolesManifest.Configuration = &configuration{}
	}
	if rolesManifest.Configuration.Templates == nil {
		rolesManifest.Configuration.Templates = map[string]string{}
	}

	for _, role := range rolesManifest.Roles {
		role.rolesManifest = &rolesManifest
		role.Jobs = make(Jobs, len(role.JobNameList))

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

		role.calculateRoleConfigurationTemplates()
	}

	return &rolesManifest, nil
}

// GetScriptPaths returns the paths to the startup / post configgin scripts for a role
func (r *Role) GetScriptPaths() map[string]string {
	result := map[string]string{}

	for _, scriptList := range [][]string{r.EnvironScripts, r.Scripts, r.PostConfigScripts} {
		for _, script := range scriptList {
			if filepath.IsAbs(script) {
				// Absolute paths _inside_ the container; there is nothing to copy
				continue
			}
			result[script] = filepath.Join(filepath.Dir(r.rolesManifest.manifestFilePath), script)
		}
	}

	return result

}

// GetRoleDevVersion gets the aggregate signature of all jobs and packages
func (r *Role) GetRoleDevVersion() string {
	roleSignature := ""
	var packages Packages

	sort.Sort(r.Jobs)
	for _, job := range r.Jobs {
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, job.SHA1)
		packages = append(packages, job.Packages...)
	}

	sort.Sort(packages)
	for _, pkg := range packages {
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, pkg.SHA1)
	}

	hasher := sha1.New()
	hasher.Write([]byte(roleSignature))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (r *Role) calculateRoleConfigurationTemplates() {
	if r.Configuration == nil {
		r.Configuration = &configuration{}
	}
	if r.Configuration.Templates == nil {
		r.Configuration.Templates = map[string]string{}
	}

	roleConfigs := map[string]string{}
	for k, v := range r.rolesManifest.Configuration.Templates {
		roleConfigs[k] = v
	}

	for k, v := range r.Configuration.Templates {
		roleConfigs[k] = v
	}

	r.Configuration.Templates = roleConfigs
}
