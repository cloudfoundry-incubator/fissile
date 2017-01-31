package kube

import (
	"io/ioutil"

	"github.com/hpcloud/fissile/model"
	"gopkg.in/yaml.v2"
)

// LoadExtendedRoleManifest loads a yaml manifest that details how jobs get grouped into roles
// This does not contain any BOSH release information
func LoadExtendedRoleManifest(manifestFilePath string) (*ExtendedRoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	rolesManifest := ExtendedRoleManifest{}

	if err := yaml.Unmarshal(manifestContents, &rolesManifest); err != nil {
		return nil, err
	}

	return &rolesManifest, nil
}

type ExtendedRoleManifest struct {
	*model.RoleManifest

	Roles         []*ExtendedRole        `yaml:"roles"`
	Configuration *ExtendedConfiguration `yaml:"configuration"`
}

type ExtendedRole struct {
	*model.Role

	Run *RoleRun `yaml:"run"`
}

type RoleRun struct {
	Scaling           *RoleRunScaling       `yaml:"scaling"`
	Capabilities      []string              `yaml:"capabilities"`
	PersistentVolumes []*RoleRunVolume      `yaml:"persistent-volumes"`
	SharedVolumes     []*RoleRunVolume      `yaml:"shared-volumes"`
	Memory            int                   `yaml:"memory"`
	VirtualCPUs       int                   `yaml:"virtual-cpus"`
	ExposedPorts      []*RoleRunExposedPort `yaml:"exposed-ports"`
}

type RoleRunScaling struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

type RoleRunVolume struct {
	Path string `yaml:"path"`
	Tag  string `yaml:"tag"`
	Size int    `yaml:"size"`
}

type RoleRunExposedPort struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	External int    `yaml:"external"`
	Internal int    `yaml:"internal"`
	Public   bool   `yaml:"public"`
}

type ExtendedConfiguration struct {
	*model.Configuration

	Variables []*ConfigurationVariable `yaml:"variables"`
}

type ConfigurationVariable struct {
	Name        string                          `yaml:"name"`
	Default     interface{}                     `yaml:"default"`
	Description string                          `yaml:"description"`
	Generator   *ConfigurationVariableGenerator `yaml:"generator"`
}

type ConfigurationVariableGenerator struct {
	Id        string `yaml:"id"`
	Type      string `yaml:"type"`
	ValueType string `yaml:"value_type"`
}
