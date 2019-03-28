package model

import (
	"fmt"
	"io/ioutil"

	"code.cloudfoundry.org/fissile/util"
	yaml "gopkg.in/yaml.v2"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	InstanceGroups InstanceGroups `yaml:"instance_groups"`
	Configuration  *Configuration `yaml:"configuration"`
	Variables      Variables
	Releases       []*ReleaseRef `yaml:"releases"`

	LoadedReleases   Releases
	Features         map[string]bool
	ManifestFilePath string
	ManifestContent  []byte `yaml:"-"`
}

// RoleManifestValidationOptions allows tests to skip some parts of validation
type RoleManifestValidationOptions struct {
	AllowMissingScripts bool
}

// LoadRoleManifestOptions provides the input to LoadRoleManifest()
type LoadRoleManifestOptions struct {
	ReleaseOptions
	Grapher           util.ModelGrapher
	ValidationOptions RoleManifestValidationOptions
}

// NewRoleManifest returns a new role manifest struct
func NewRoleManifest() *RoleManifest {
	m := &RoleManifest{}
	m.Features = make(map[string]bool)
	return m
}

// LoadManifestFromFile loads the manifest content from a file
func (m *RoleManifest) LoadManifestFromFile(manifestFilePath string) (err error) {
	m.ManifestContent, err = ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return
	}
	m.ManifestFilePath = manifestFilePath
	err = yaml.Unmarshal(m.ManifestContent, &m)
	return
}

// AddFeature will add a feature name to the manifest.
// A feature needs to be enabled only once to be enabled globally.
func (m *RoleManifest) AddFeature(name string, enabledByDefault bool) {
	if name != "" {
		if _, exists := m.Features[name]; !exists || enabledByDefault {
			m.Features[name] = enabledByDefault
		}
	}
}

// LookupInstanceGroup will find the given instance group in the role manifest
func (m *RoleManifest) LookupInstanceGroup(name string) *InstanceGroup {
	for _, instanceGroup := range m.InstanceGroups {
		if instanceGroup.Name == name {
			return instanceGroup
		}
	}
	return nil
}

// SelectInstanceGroups will find only the given instance groups in the role manifest
func (m *RoleManifest) SelectInstanceGroups(roleNames []string) (InstanceGroups, error) {
	if len(roleNames) == 0 {
		// No names specified, assume all instance groups
		return m.InstanceGroups, nil
	}

	var results InstanceGroups
	var missingRoles []string

	for _, roleName := range roleNames {
		if instanceGroup := m.LookupInstanceGroup(roleName); instanceGroup != nil {
			results = append(results, instanceGroup)
		} else {
			missingRoles = append(missingRoles, roleName)
		}
	}
	if len(missingRoles) > 0 {
		return nil, fmt.Errorf("Some instance groups are unknown: %v", missingRoles)
	}

	return results, nil
}

// GetTemplate returns a property from a yaml.MapSlice
func GetTemplate(propertyDefs yaml.MapSlice, property string) (interface{}, bool) {
	for _, item := range propertyDefs {
		if item.Key.(string) == property {
			return item.Value, true
		}
	}

	return "", false
}
