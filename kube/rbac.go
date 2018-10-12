package kube

import (
	"fmt"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
)

// NewRBACAccount creates a new (Kubernetes RBAC) service account and associated
// bindings.
func NewRBACAccount(name string, account model.AuthAccount, settings ExportSettings) ([]helm.Node, error) {
	var resources []helm.Node
	block := authModeRBAC(settings)

	// If we want to modify the default account, there's no need to create it
	// first -- it already exists
	if name != "default" {
		resources = append(resources, newKubeConfig(settings, "v1", "ServiceAccount", name, block))
	}

	for _, role := range account.Roles {
		binding := newKubeConfig(settings, "rbac.authorization.k8s.io/v1beta1", "RoleBinding", fmt.Sprintf("%s-%s-binding", name, role), block)
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

	// We have no proper namespace default for kube configuration.
	namespace := "~"
	if settings.CreateHelmChart {
		namespace = "{{ .Release.Namespace }}"
	}

	binding := newKubeConfig(settings, "rbac.authorization.k8s.io/v1", "ClusterRoleBinding",
		authCRBindingName(name, settings),
		authPSPCondition(account.PodSecurityPolicy, settings))
	subjects := helm.NewList(helm.NewMapping(
		"kind", "ServiceAccount",
		"name", name,
		"namespace", namespace))
	binding.Add("subjects", subjects)
	binding.Add("roleRef", helm.NewMapping(
		"kind", "ClusterRole",
		"name", authPSPRoleName(account.PodSecurityPolicy, settings),
		"apiGroup", "rbac.authorization.k8s.io"))
	resources = append(resources, binding)

	return resources, nil
}

// NewRBACRole creates a new (Kubernetes RBAC) role
func NewRBACRole(name string, authRole model.AuthRole, settings ExportSettings) (helm.Node, error) {
	rules := helm.NewList()
	for _, ruleSpec := range authRole {
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

	role := newKubeConfig(settings, "rbac.authorization.k8s.io/v1beta1", "Role", name, authModeRBAC(settings))
	role.Add("rules", rules)

	return role.Sort(), nil
}

func authModeRBAC(settings ExportSettings) helm.NodeModifier {
	if settings.CreateHelmChart {
		return helm.Block(`if eq (printf "%s" .Values.kube.auth) "rbac"`)
	}
	return nil
}

// authPSPCondition creates a block condition checking for RBAC and the named PSP
func authPSPCondition(psp string, settings ExportSettings) helm.NodeModifier {
	if settings.CreateHelmChart {
		return helm.Block(fmt.Sprintf(`if and (%s) .Values.kube.psp.%s`,
			`eq (printf "%s" .Values.kube.auth) "rbac"`, psp))
	}
	return nil
}

// authPSPRoleName derives the name of the cluster role for a PSP
func authPSPRoleName(psp string, settings ExportSettings) string {
	if settings.CreateHelmChart {
		return fmt.Sprintf("{{ .Release.Namespace }}-psp-role-%s", psp)
	}
	return fmt.Sprintf("psp-role-%s", psp)
}

// authCRBindingName derives the name of the cluster role for a PSP
func authCRBindingName(name string, settings ExportSettings) string {
	if settings.CreateHelmChart {
		return fmt.Sprintf("{{ .Release.Namespace }}-%s-binding-psp", name)
	}
	return fmt.Sprintf("%s-binding-psp", name)
}

// NewRBACClusterRolePSP creates a new (Kubernetes RBAC) cluster role
// referencing a pod security policy (PSP)
func NewRBACClusterRolePSP(psp string, settings ExportSettings) (helm.Node, error) {
	name := authPSPRoleName(psp, settings)

	clusterRole := newKubeConfig(settings, "rbac.authorization.k8s.io/v1", "ClusterRole", name, authPSPCondition(psp, settings))

	if settings.CreateHelmChart {
		psp = fmt.Sprintf("{{ .Values.kube.psp.%s | quote }}", psp)
	}

	rules := helm.NewList()
	rule := helm.NewMapping()
	rule.Add("apiGroups", helm.NewList("extensions"))
	rule.Add("resources", helm.NewList("podsecuritypolicies"))
	rule.Add("verbs", helm.NewList("use"))
	rule.Add("resourceNames", helm.NewList(psp))
	rules.Add(rule.Sort())

	clusterRole.Add("rules", rules)

	return clusterRole.Sort(), nil
}
