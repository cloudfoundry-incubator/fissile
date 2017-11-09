package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

const authModeRBAC = `if eq (printf "%s" .Values.kube.auth) "rbac"`

// NewRBACAccount creates a new (Kubernetes RBAC) service account and associated
// bindings.
func NewRBACAccount(name string, account model.AuthAccount, settings ExportSettings) ([]helm.Node, error) {
	block := helm.Block("")
	if settings.CreateHelmChart {
		block = helm.Block(authModeRBAC)
	}

	var resources []helm.Node

	// If we want to modify the default account, there's no need to create it
	// first -- it already exists
	if name != "default" {
		accountYAML := newTypeMeta("v1", "ServiceAccount", block)
		accountYAML.Add("metadata", helm.NewMapping("name", name))
		resources = append(resources, accountYAML)
	}

	for _, role := range account.Roles {
		binding := newTypeMeta("rbac.authorization.k8s.io/v1beta1", "RoleBinding", block)
		binding.Add("metadata", helm.NewMapping("name", fmt.Sprintf("%s-%s-binding", name, role)))
		subjects := helm.NewList(helm.NewMapping(
			"kind", "ServiceAccount",
			"name", name))
		binding.Add("subjects", subjects)
		binding.Add("roleRef", helm.NewMapping(
			"kind", "Role",
			"name", role,
			"apiGroup", "rbac.authorization.k8s.io"))
		resources = append(resources, binding)
	}

	return resources, nil
}

// NewRBACRole creates a new (Kubernetes RBAC) role
func NewRBACRole(name string, role model.AuthRole, settings ExportSettings) (helm.Node, error) {
	rules := helm.NewList()
	for _, ruleSpec := range role {
		rule := helm.NewMapping()
		APIGroups := helm.NewList()
		for _, APIGroup := range ruleSpec.APIGroups {
			APIGroups.Add(APIGroup)
		}
		rule.Add("apiGroups", APIGroups)
		resources := helm.NewList()
		for _, resource := range ruleSpec.Resources {
			resources.Add(resource)
		}
		rule.Add("resources", resources)
		verbs := helm.NewList()
		for _, verb := range ruleSpec.Verbs {
			verbs.Add(verb)
		}
		rule.Add("verbs", verbs)
		rules.Add(rule.Sort())
	}

	container := newTypeMeta("rbac.authorization.k8s.io/v1beta1", "Role")
	if settings.CreateHelmChart {
		container.Set(helm.Block(authModeRBAC))
	}
	container.Add("metadata", helm.NewMapping("name", name))
	container.Add("rules", rules)

	return container.Sort(), nil
}
