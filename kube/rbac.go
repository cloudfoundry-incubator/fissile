package kube

import (
	"fmt"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
)

// RBACRoleKind enumerations are for NewRBACRole
type RBACRoleKind string

const (
	// RBACRoleKindRole are (namespaced) roles
	RBACRoleKindRole RBACRoleKind = "Role"
	// RBACRoleKindClusterRole are cluster roles
	RBACRoleKindClusterRole RBACRoleKind = "ClusterRole"
)

// NewRBACAccount creates a new (Kubernetes RBAC) service account and associated
// bindings.
func NewRBACAccount(accountName string, config *model.Configuration, settings ExportSettings) ([]helm.Node, error) {
	var resources []helm.Node
	block := authModeRBAC(settings)

	account, ok := config.Authorization.Accounts[accountName]
	if !ok {
		return nil, fmt.Errorf("Account %s not found", accountName)
	}

	if len(account.UsedBy) < 1 {
		// Nothing uses this account
		// Possibly, we generated a privileged version instead
		return nil, nil
	}

	// If we want to modify the default account, there's no need to create it
	// first -- it already exists
	if accountName != "default" {
		description := fmt.Sprintf("Service account %s is used by the following instance groups:", accountName)
		for instanceGroupName := range account.UsedBy {
			description += fmt.Sprintf("\n - %s", instanceGroupName)
		}
		resources = append(resources, newKubeConfig(settings,
			"v1",
			"ServiceAccount",
			accountName,
			block,
			helm.Comment(description)))
	}

	// For each role, create a role binding
	for _, roleName := range account.Roles {
		// Embed the role first, if it's only used by this binding
		var usedByAccounts []string
		for accountName := range config.Authorization.RoleUsedBy[roleName] {
			usedByAccounts = append(usedByAccounts, fmt.Sprintf("- %s", accountName))
		}
		if len(usedByAccounts) < 2 {
			role, err := NewRBACRole(
				roleName,
				RBACRoleKindRole,
				config.Authorization.Roles[roleName],
				settings)
			if err != nil {
				return nil, err
			}
			role.Set(helm.Comment(fmt.Sprintf(`Role %s only used by account %s`, roleName, usedByAccounts)))
			resources = append(resources, role)
		}

		binding := newKubeConfig(settings,
			"rbac.authorization.k8s.io/v1",
			"RoleBinding",
			fmt.Sprintf("%s-%s-binding", accountName, roleName),
			block,
			helm.Comment(fmt.Sprintf("Role binding for service account %s and role %s", accountName, roleName)))
		subjects := helm.NewList(helm.NewMapping(
			"kind", "ServiceAccount",
			"name", accountName))
		binding.Add("subjects", subjects)
		binding.Add("roleRef", helm.NewMapping(
			"apiGroup", "rbac.authorization.k8s.io",
			"kind", "Role",
			"name", roleName))
		resources = append(resources, binding)
	}

	// We have no proper namespace default for kube configuration.
	namespace := "~"
	if settings.CreateHelmChart {
		namespace = "{{ .Release.Namespace }}"
	}

	// For each cluster role, create a cluster role binding
	// And if the cluster role is only used here, embed that too
	for _, clusterRoleName := range account.ClusterRoles {
		// Embed the cluster role first, if it's only used by this binding
		var accountNames []string
		for accountName := range config.Authorization.ClusterRoleUsedBy[clusterRoleName] {
			accountNames = append(accountNames, accountName)
		}
		if len(accountNames) < 2 {
			role, err := NewRBACRole(
				clusterRoleName,
				RBACRoleKindClusterRole,
				config.Authorization.ClusterRoles[clusterRoleName],
				settings)
			if err != nil {
				return nil, err
			}
			role.Set(helm.Comment(fmt.Sprintf(`Cluster role %s only used by account %s`, clusterRoleName, accountNames)))
			resources = append(resources, role)
		}

		binding := newKubeConfig(settings,
			"rbac.authorization.k8s.io/v1",
			"ClusterRoleBinding",
			authCRBindingName(accountName, clusterRoleName, settings),
			block,
			helm.Comment(fmt.Sprintf("Cluster role binding for service account %s and cluster role %s", accountName, clusterRoleName)))
		subjects := helm.NewList(helm.NewMapping(
			"kind", "ServiceAccount",
			"name", accountName,
			"namespace", namespace))
		binding.Add("subjects", subjects)
		binding.Add("roleRef", helm.NewMapping(
			"kind", "ClusterRole",
			"name", authCRName(clusterRoleName, settings),
			"apiGroup", "rbac.authorization.k8s.io"))
		resources = append(resources, binding)
	}

	return resources, nil
}

// NewRBACRole creates a new (Kubernetes RBAC) role / cluster role
func NewRBACRole(name string, kind RBACRoleKind, authRole model.AuthRole, settings ExportSettings) (helm.Node, error) {
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
		if len(ruleSpec.ResourceNames) > 0 {
			resourceNames := helm.NewList()
			for _, resourceName := range ruleSpec.ResourceNames {
				if settings.CreateHelmChart && ruleSpec.IsPodSecurityPolicyRule() {
					// When creating helm charts for PSPs, let the user override it
					resourceNames.Add(fmt.Sprintf(
						`{{ default (printf "%%s-psp-%s" .Release.Namespace) .Values.kube.psp.%s }}`,
						resourceName,
						resourceName))
				} else {
					resourceNames.Add(resourceName)
				}
			}
			rule.Add("resourceNames", resourceNames)
		}
		rules.Add(rule.Sort())
	}

	if kind == RBACRoleKindClusterRole {
		name = authCRName(name, settings)
	}
	role := newKubeConfig(settings, "rbac.authorization.k8s.io/v1", string(kind), name, authModeRBAC(settings))
	role.Add("rules", rules)

	return role.Sort(), nil
}

// NewRBACPSP creates a (Kubernetes RBAC) pod security policy
func NewRBACPSP(name string, psp *model.PodSecurityPolicy, settings ExportSettings) (helm.Node, error) {
	var condition helm.NodeModifier
	if settings.CreateHelmChart {
		condition = helm.Block(fmt.Sprintf(`if and (%s) (not .Values.kube.psp.%s)`,
			`eq (printf "%s" .Values.kube.auth) "rbac"`, name))
		name = fmt.Sprintf(`{{ printf "%%s-psp-%s" .Release.Namespace }}`, name)
	}
	node := newKubeConfig(settings,
		"extensions/v1beta1",
		"PodSecurityPolicy",
		name,
		condition)
	node.Add("spec", helm.NewNode(psp.Definition))
	return node, nil
}

// authModeRBAC returns a block condition checking for RBAC
func authModeRBAC(settings ExportSettings) helm.NodeModifier {
	if settings.CreateHelmChart {
		return helm.Block(fmt.Sprintf(
			`if and (%s) (%s)`,
			`eq (printf "%s" .Values.kube.auth) "rbac"`,
			`.Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1"`))
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
func authCRBindingName(name, clusterRoleName string, settings ExportSettings) string {
	if settings.CreateHelmChart {
		return fmt.Sprintf("{{ .Release.Namespace }}-%s-%s-cluster-binding", name, clusterRoleName)
	}
	return fmt.Sprintf("%s-%s-cluster-binding", name, clusterRoleName)
}

// authCRName derives the cluster role name from the name space
func authCRName(name string, settings ExportSettings) string {
	if settings.CreateHelmChart {
		return fmt.Sprintf(`{{ .Release.Namespace }}-cluster-role-%s`, name)
	}
	return name
}
