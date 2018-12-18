package model

import (
	yaml "gopkg.in/yaml.v2"
)

// Configuration contains information about how to configure the
// resulting images
type Configuration struct {
	Authorization struct {
		RoleUse  map[string]int
		Roles    map[string]AuthRole    `yaml:"roles,omitempty"`
		Accounts map[string]AuthAccount `yaml:"accounts,omitempty"`
	} `yaml:"auth,omitempty"`
	Templates yaml.MapSlice `yaml:"templates"`
}

// Notes: It was decided to use a separate `RoleUse` map to hold the
// usage count for the roles to keep the API to the role manifest
// yaml.  Going to a structure for AuthRole, with a new field for the
// counter would change the structure of the yaml as well.

// An AuthRule is a single rule for a RBAC authorization role
type AuthRule struct {
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

// An AuthRole is a role for RBAC authorization
type AuthRole []AuthRule

// An AuthAccount is a service account for RBAC authorization
// The NumGroups field records the number of instance groups
// referencing the account in question.
type AuthAccount struct {
	NumGroups         int
	Roles             []string `yaml:"roles"`
	PodSecurityPolicy string
}
