package model

import (
	"gopkg.in/yaml.v2"
)

// Configuration contains information about how to configure the
// resulting images
type Configuration struct {
	Authorization struct {
		Roles    map[string]AuthRole    `yaml:"roles,omitempty"`
		Accounts map[string]AuthAccount `yaml:"accounts,omitempty"`
	} `yaml:"auth,omitempty"`
	Templates yaml.MapSlice `yaml:"templates"`
}

// An AuthRule is a single rule for a RBAC authorization role
type AuthRule struct {
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

// An AuthRole is a role for RBAC authorization
type AuthRole []AuthRule

// An AuthAccount is a service account for RBAC authorization
type AuthAccount struct {
	Roles             []string `yaml:"roles"`
	PodSecurityPolicy string
}
