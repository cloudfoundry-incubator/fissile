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

	// Integrate a PSP, via ClusterRoleBinding to ClusterRole
	if account.PodSecurityPolicy != "" && account.PodSecurityPolicy != "null" {
		blockPSP := helm.Block("")

		// We have no proper namespace default for kube configuration.
		namespace := "~"
		if settings.CreateHelmChart {
			blockPSP = authPSPCondition(account.PodSecurityPolicy)
			namespace = "{{ .Release.Namespace }}"
		}

		binding := newTypeMeta("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", blockPSP)
		binding.Add("metadata", helm.NewMapping("name", fmt.Sprintf("%s-binding-psp", name)))
		subjects := helm.NewList(helm.NewMapping(
			"kind", "ServiceAccount",
			"name", name,
			"namespace", namespace))
		binding.Add("subjects", subjects)
		binding.Add("roleRef", helm.NewMapping(
			"kind", "ClusterRole",
			"name", authPSPRoleName(account.PodSecurityPolicy),
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

// authPSPCondition creates a block condition checking for RBAC and the named PSP
func authPSPCondition(psp string) helm.NodeModifier {
	return helm.Block(fmt.Sprintf(`if and (%s) .Values.kube.psp.%s`,
		`eq (printf "%s" .Values.kube.auth) "rbac"`, psp))
}

// authPSPRoleName derives the name of the cluster role for a PSP
func authPSPRoleName(psp string) string {
	return fmt.Sprintf("psp-role-%s", psp)
}

// NewRBACClusterRolePSP creates a new (Kubernetes RBAC) cluster role
// referencing a pod security policy (PSP)
func NewRBACClusterRolePSP(psp string, settings ExportSettings) (helm.Node, error) {
	name := authPSPRoleName(psp)

	container := newTypeMeta("rbac.authorization.k8s.io/v1", "ClusterRole")

	if settings.CreateHelmChart {
		container.Set(authPSPCondition(psp))
		psp = fmt.Sprintf("{{ .Values.kube.psp.%s | quote }}", psp)
	}

	rules := helm.NewList()
	rule := helm.NewMapping()
	rule.Add("apiGroups", helm.NewList("extensions"))
	rule.Add("resources", helm.NewList("podsecuritypolicies"))
	rule.Add("verbs", helm.NewList("use"))
	rule.Add("resourceNames", helm.NewList(psp))
	rules.Add(rule.Sort())

	container.Add("metadata", helm.NewMapping("name", name))
	container.Add("rules", rules)

	return container.Sort(), nil
}
