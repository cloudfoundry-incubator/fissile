package model

import (
	"code.cloudfoundry.org/fissile/util"
	yaml "gopkg.in/yaml.v2"
)

// Configuration contains information about how to configure the
// resulting images
type Configuration struct {
	Authorization ConfigurationAuthorization `yaml:"auth,omitempty"`
	Templates     yaml.MapSlice              `yaml:"templates"`
}

// ConfigurationAuthorization defines Configuration.Authorization
type ConfigurationAuthorization struct {
	RoleUsedBy          map[string]map[string]struct{} `yaml:"-"`
	Roles               map[string]AuthRole            `yaml:"roles,omitempty"`
	ClusterRoles        map[string]AuthRole            `yaml:"cluster-roles,omitempty"`
	ClusterRoleUsedBy   map[string]map[string]struct{} `yaml:"-"`
	PodSecurityPolicies map[string]*PodSecurityPolicy  `yaml:"pod-security-policies,omitempty"`
	Accounts            map[string]AuthAccount         `yaml:"accounts,omitempty"`
}

// Notes: It was decided to use a separate `RoleUse` map to hold the
// usage count for the roles to keep the API to the role manifest
// yaml.  Going to a structure for AuthRole, with a new field for the
// counter would change the structure of the yaml as well.

// An AuthRule is a single rule for a RBAC authorization role
type AuthRule struct {
	APIGroups     []string `yaml:"apiGroups"`
	Resources     []string `yaml:"resources"`
	ResourceNames []string `yaml:"resourceNames"`
	Verbs         []string `yaml:"verbs"`
}

// IsPodSecurityPolicyRule checks if the rule is a pod security policy rule
func (rule *AuthRule) IsPodSecurityPolicyRule() bool {
	isCorrectAPIGroup := false
	for _, group := range []string{"extensions", "policy"} {
		if util.StringInSlice(group, rule.APIGroups) {
			isCorrectAPIGroup = true
			break
		}
	}
	if !isCorrectAPIGroup {
		return false
	}
	if !util.StringInSlice("use", rule.Verbs) {
		return false
	}
	if !util.StringInSlice("podsecuritypolicies", rule.Resources) {
		return false
	}
	return true
}

// An AuthRole is a role for RBAC authorization
type AuthRole []AuthRule

// An AuthAccount is a service account for RBAC authorization
// The NumGroups field records the number of instance groups
// referencing the account in question.
type AuthAccount struct {
	Roles        []string `yaml:"roles"`
	ClusterRoles []string `yaml:"cluster-roles"`

	UsedBy map[string]struct{} `yaml:"-"` // Instance groups which use this account
}
