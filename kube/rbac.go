package kube

import (
	"fmt"
	"sort"
	"strings"

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
		var instanceGroupNames []string
		for instanceGroupName := range account.UsedBy {
			instanceGroupNames = append(instanceGroupNames, fmt.Sprintf("- %s", instanceGroupName))
		}
		sort.Strings(instanceGroupNames)
		description := fmt.Sprintf(
			"Service account \"%s\" is used by the following instance groups:\n%s",
			accountName,
			strings.Join(instanceGroupNames, "\n"))

		cb := NewConfigBuilder().
			SetSettings(&settings).
			SetAPIVersion("v1").
			SetKind("ServiceAccount").
			SetName(accountName).
			AddModifier(block).
			AddModifier(helm.Comment(description))
		serviceAccount, err := cb.Build()
		if err != nil {
			return nil, fmt.Errorf("failed to build a new kube config: %v", err)
		}
		resources = append(resources, serviceAccount)
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
			role.Set(helm.Comment(fmt.Sprintf(`Role "%s" only used by account "%s"`, roleName, usedByAccounts)))
			resources = append(resources, role)
		}

		cb := NewConfigBuilder().
			SetSettings(&settings).
			SetAPIVersion("rbac.authorization.k8s.io/v1").
			SetKind("RoleBinding").
			SetName(fmt.Sprintf("%s-%s-binding", accountName, roleName)).
			AddModifier(block).
			AddModifier(helm.Comment(fmt.Sprintf(`Role binding for service account "%s" and role "%s"`,
				accountName,
				roleName)))
		binding, err := cb.Build()
		if err != nil {
			return nil, fmt.Errorf("failed to build a new kube config: %v", err)
		}
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
			role.Set(helm.Comment(fmt.Sprintf(`Cluster role "%s" only used by account "%s"`, clusterRoleName, accountNames)))
			resources = append(resources, role)
		}

		cb := NewConfigBuilder().
			SetSettings(&settings).
			SetAPIVersion("rbac.authorization.k8s.io/v1").
			SetKind("ClusterRoleBinding").
			AddModifier(block).
			AddModifier(helm.Comment(fmt.Sprintf(`Cluster role binding for service account "%s" and cluster role "%s"`,
				accountName,
				clusterRoleName)))
		if settings.CreateHelmChart {
			cb.SetNameHelmExpression(
				fmt.Sprintf(`{{ template "fissile.SanitizeName" (printf "%%s-%s-%s-cluster-binding" .Release.Namespace) }}`,
					accountName,
					clusterRoleName))
		} else {
			cb.SetName(fmt.Sprintf("%s-%s-cluster-binding", accountName, clusterRoleName))
		}
		binding, err := cb.Build()
		if err != nil {
			return nil, fmt.Errorf("failed to build a new kube config: %v", err)
		}
		subjects := helm.NewList(helm.NewMapping(
			"kind", "ServiceAccount",
			"name", accountName,
			"namespace", namespace))
		binding.Add("subjects", subjects)
		roleRef := helm.NewMapping(
			"kind", "ClusterRole",
			"apiGroup", "rbac.authorization.k8s.io")
		if settings.CreateHelmChart {
			roleRef.Add("name", fmt.Sprintf(`{{ template "fissile.SanitizeName" (printf "%%s-cluster-role-%s" .Release.Namespace) }}`, clusterRoleName))
		} else {
			roleRef.Add("name", clusterRoleName)
		}
		binding.Add("roleRef", roleRef)
		resources = append(resources, binding)
	}

	return resources, nil
}

// NewRBACRole creates a new (Kubernetes RBAC) role / cluster role
func NewRBACRole(name string, kind RBACRoleKind, authRole model.AuthRole, settings ExportSettings) (helm.Node, error) {
	rules := helm.NewList()
	for _, ruleSpec := range authRole {
		rule := helm.NewMapping()
		rule.Add("apiGroups", helm.NewNode(ruleSpec.APIGroups))
		rule.Add("resources", helm.NewNode(ruleSpec.Resources))
		rule.Add("verbs", helm.NewNode(ruleSpec.Verbs))
		if len(ruleSpec.ResourceNames) > 0 {
			resourceNames := helm.NewList()
			for _, resourceName := range ruleSpec.ResourceNames {
				if settings.CreateHelmChart && ruleSpec.IsPodSecurityPolicyRule() {
					// When creating helm charts for PSPs, let the user override it
					resourceNames.Add(fmt.Sprintf(
						`{{ if .Values.kube.psp.%[1]s }}{{ .Values.kube.psp.%[1]s }}{{ else }}`+
							`{{ template "fissile.SanitizeName" (printf "%%s-psp-%[1]s" .Release.Namespace) }}{{ end }}`, resourceName))
				} else {
					resourceNames.Add(resourceName)
				}
			}
			rule.Add("resourceNames", resourceNames)
		}
		rules.Add(rule.Sort())
	}

	cb := NewConfigBuilder().
		SetSettings(&settings).
		SetAPIVersion("rbac.authorization.k8s.io/v1").
		SetKind(string(kind)).
		AddModifier(authModeRBAC(settings))
	if kind == RBACRoleKindClusterRole && settings.CreateHelmChart {
		cb.SetNameHelmExpression(fmt.Sprintf(`{{ template "fissile.SanitizeName" (printf "%%s-cluster-role-%s" .Release.Namespace) }}`, name))
	} else {
		cb.SetName(name)
	}
	role, err := cb.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build a new kube config: %v", err)
	}
	role.Add("rules", rules)

	return role.Sort(), nil
}

// NewRBACPSP creates a (Kubernetes RBAC) pod security policy
func NewRBACPSP(name string, psp *model.PodSecurityPolicy, settings ExportSettings) (helm.Node, error) {
	cb := NewConfigBuilder().
		SetSettings(&settings).
		SetConditionalAPIVersion("policy/v1beta1", "extensions/v1beta1").
		SetKind("PodSecurityPolicy").
		AddModifier(helm.Comment(fmt.Sprintf(`Pod security policy "%s"`, name)))
	if settings.CreateHelmChart {
		cb.AddModifier(helm.Block(fmt.Sprintf(`if and (%s) (not .Values.kube.psp.%s)`,
			`eq (printf "%s" .Values.kube.auth) "rbac"`, name)))
		cb.SetNameHelmExpression(fmt.Sprintf(`{{ template "fissile.SanitizeName" (printf "%%s-psp-%s" .Release.Namespace) }}`, name))
	} else {
		cb.SetName(name)
	}
	node, err := cb.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build a new kube config: %v", err)
	}
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
