package kube

import (
	"testing"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRBACAccountPSPKube(t *testing.T) {
	t.Parallel()

	resources, err := NewRBACAccount("the-name",
		&model.Configuration{
			Authorization: model.ConfigurationAuthorization{
				Accounts: map[string]model.AuthAccount{
					"the-name": {
						Roles:        []string{"a-role"},
						ClusterRoles: []string{"privileged-cluster-role"},
						UsedBy: map[string]struct{}{
							// This must be used by multiple instance groups to be serialized
							"foo": struct{}{},
							"bar": struct{}{},
						},
					},
				},
			},
		}, ExportSettings{})

	require.NoError(t, err)

	account := matchNodeInList(resources, helm.NewMapping("kind", "ServiceAccount"))
	if assert.NotNil(t, account, "service account not found") {
		actualAccount, err := RoundtripKube(account)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
			apiVersion: "v1"
			kind: "ServiceAccount"
			metadata:
				name: "the-name"
				labels:
					app.kubernetes.io/component: the-name
		`, actualAccount)
		}
	}

	roleBinding := matchNodeInList(resources, helm.NewMapping("kind", "RoleBinding"))
	if assert.NotNil(t, roleBinding, "role binding not found") {
		actualRole, err := RoundtripKube(roleBinding)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: "rbac.authorization.k8s.io/v1"
				kind: "RoleBinding"
				metadata:
					name: "the-name-a-role-binding"
					labels:
						app.kubernetes.io/component: the-name-a-role-binding
				subjects:
				-	kind: "ServiceAccount"
					name: "the-name"
				roleRef:
					kind: "Role"
					name: "a-role"
					apiGroup: "rbac.authorization.k8s.io"
			`, actualRole)
		}
	}

	role := matchNodeInList(resources, helm.NewMapping("kind", "Role"))
	if assert.NotNil(t, role, "role not found") {
		actualRole, err := RoundtripKube(role)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: Role
				metadata:
					labels:
						app.kubernetes.io/component: a-role
					name: a-role
				rules: []
			`, actualRole)
		}
	}

	clusterRoleBinding := matchNodeInList(resources, helm.NewMapping("kind", "ClusterRoleBinding"))
	if assert.NotNil(t, clusterRoleBinding, "cluster role binding not found") {
		actualBinding, err := RoundtripKube(clusterRoleBinding)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
			apiVersion: "rbac.authorization.k8s.io/v1"
			kind: "ClusterRoleBinding"
			metadata:
				name: "the-name-privileged-cluster-role-cluster-binding"
				labels:
					app.kubernetes.io/component: the-name-privileged-cluster-role-cluster-binding
			subjects:
			-	kind: "ServiceAccount"
				name: "the-name"
				namespace: "~"
			roleRef:
				kind: "ClusterRole"
				name: "privileged-cluster-role"
				apiGroup: "rbac.authorization.k8s.io"
		`, actualBinding)
		}
	}

	clusterRole := matchNodeInList(resources, helm.NewMapping("kind", "ClusterRole"))
	if assert.NotNil(t, clusterRole, "cluster role not found") {
		actualClusterRole, err := RoundtripKube(clusterRole)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: ClusterRole
				metadata:
					labels:
						app.kubernetes.io/component: privileged-cluster-role
					name: privileged-cluster-role
				rules: []
			`, actualClusterRole)
		}
	}

}

func TestNewRBACAccountHelm(t *testing.T) {
	t.Parallel()

	resources, err := NewRBACAccount("the-name",
		&model.Configuration{
			Authorization: model.ConfigurationAuthorization{
				Accounts: map[string]model.AuthAccount{
					"the-name": model.AuthAccount{
						Roles:        []string{"a-role"},
						ClusterRoles: []string{"nonprivileged"},
						UsedBy: map[string]struct{}{
							// This must be used by multiple instance groups to be serialized
							"foo": struct{}{},
							"bar": struct{}{},
						},
					},
				},
				ClusterRoles: map[string]model.AuthRole{
					"nonprivileged": {
						{
							APIGroups:     []string{"policy"},
							Resources:     []string{"podsecuritypolicies"},
							ResourceNames: []string{"nonprivileged"},
							Verbs:         []string{"use"},
						},
						{
							APIGroups:     []string{"imaginary"},
							Resources:     []string{"other"},
							ResourceNames: []string{"unchanged"},
							Verbs:         []string{"yank"},
						},
					},
				},
			},
		}, ExportSettings{
			CreateHelmChart: true,
		})

	require.NoError(t, err)
	require.Len(t, resources, 5, "Should have account, role binding, and cluster role binding")

	account := matchNodeInList(resources, helm.NewMapping("kind", "ServiceAccount"))
	roleBinding := matchNodeInList(resources, helm.NewMapping("kind", "RoleBinding"))
	role := matchNodeInList(resources, helm.NewMapping("kind", "Role"))
	clusterRoleBinding := matchNodeInList(resources, helm.NewMapping("kind", "ClusterRoleBinding"))
	clusterRole := matchNodeInList(resources, helm.NewMapping("kind", "ClusterRole"))

	t.Run("NoAuth", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth": "",
		}
		actualAccount, err := RoundtripNode(account, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
			`, actualAccount)
		}

		actualRoleBinding, err := RoundtripNode(roleBinding, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
			`, actualRoleBinding)
		}

		actualRole, err := RoundtripNode(role, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
			`, actualRole)
		}

		actualClusterRoleBinding, err := RoundtripNode(clusterRoleBinding, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
			`, actualClusterRoleBinding)
		}

		actualClusterRole, err := RoundtripNode(clusterRole, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
			`, actualClusterRole)
		}
	})

	t.Run("DefaultPodSecurityPolicy", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth":              "rbac",
			"Values.kube.psp.nonprivileged": nil,
			"Release.Namespace":             "default-namespace",
		}
		actualAccount, err := RoundtripNode(account, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert.New(t), `---
			apiVersion: "v1"
			kind: "ServiceAccount"
			metadata:
				name: "the-name"
				labels:
					app.kubernetes.io/component: the-name
					app.kubernetes.io/instance: MyRelease
					app.kubernetes.io/managed-by: Tiller
					app.kubernetes.io/name: MyChart
					app.kubernetes.io/version: 1.22.333.4444
					helm.sh/chart: MyChart-42.1_foo
					skiff-role-name: "the-name"
		`, actualAccount)

		actualRoleBinding, err := RoundtripNode(roleBinding, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: RoleBinding
				metadata:
					labels:
						app.kubernetes.io/component: the-name-a-role-binding
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: the-name-a-role-binding
					name: the-name-a-role-binding
				roleRef:
					apiGroup: rbac.authorization.k8s.io
					kind: Role
					name: a-role
				subjects:
				-	kind: ServiceAccount
					name: the-name
			`, actualRoleBinding)
		}

		actualRole, err := RoundtripNode(role, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: Role
				metadata:
					labels:
						app.kubernetes.io/component: a-role
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: a-role
					name: a-role
				rules: []
			`, actualRole)
		}

		actualClusterRoleBinding, err := RoundtripNode(clusterRoleBinding, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: ClusterRoleBinding
				metadata:
					labels:
						app.kubernetes.io/component: default-namespace-the-name-nonprivileged-cluster-binding
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: default-namespace-the-name-nonprivileged-cluster-binding
					name: default-namespace-the-name-nonprivileged-cluster-binding
				roleRef:
					apiGroup: rbac.authorization.k8s.io
					kind: ClusterRole
					name: default-namespace-cluster-role-nonprivileged
				subjects:
				-	kind: ServiceAccount
					name: the-name
					namespace: default-namespace
			`, actualClusterRoleBinding)
		}

		actualClusterRole, err := RoundtripNode(clusterRole, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: ClusterRole
				metadata:
					labels:
						app.kubernetes.io/component: default-namespace-cluster-role-nonprivileged
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: default-namespace-cluster-role-nonprivileged
					name: default-namespace-cluster-role-nonprivileged
				rules:
				-	apiGroups: [policy]
					resources: [podsecuritypolicies]
					verbs: [use]
					resourceNames: [default-namespace-psp-nonprivileged]
				-	apiGroups:     [imaginary]
					resourceNames: [unchanged]
					resources:     [other]
					verbs:         [yank]
			`, actualClusterRole)
		}
	})

	t.Run("OverridePodSecurityPolicy", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth":              "rbac",
			"Values.kube.psp.nonprivileged": "psp-name",
			"Release.Namespace":             "a-namespace",
		}

		actualAccount, err := RoundtripNode(account, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert.New(t), `---
			apiVersion: "v1"
			kind: "ServiceAccount"
			metadata:
				name: "the-name"
				labels:
					app.kubernetes.io/component: the-name
					app.kubernetes.io/instance: MyRelease
					app.kubernetes.io/managed-by: Tiller
					app.kubernetes.io/name: MyChart
					app.kubernetes.io/version: 1.22.333.4444
					helm.sh/chart: MyChart-42.1_foo
					skiff-role-name: "the-name"
		`, actualAccount)

		actualRoleBinding, err := RoundtripNode(roleBinding, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: RoleBinding
				metadata:
					labels:
						app.kubernetes.io/component: the-name-a-role-binding
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: the-name-a-role-binding
					name: the-name-a-role-binding
				roleRef:
					apiGroup: rbac.authorization.k8s.io
					kind: Role
					name: a-role
				subjects:
				-	kind: ServiceAccount
					name: the-name
			`, actualRoleBinding)
		}

		actualRole, err := RoundtripNode(role, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: Role
				metadata:
					labels:
						app.kubernetes.io/component: a-role
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: a-role
					name: a-role
				rules: []
			`, actualRole)
		}

		actualClusterRoleBinding, err := RoundtripNode(clusterRoleBinding, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: "rbac.authorization.k8s.io/v1"
				kind: "ClusterRoleBinding"
				metadata:
					name: "a-namespace-the-name-nonprivileged-cluster-binding"
					labels:
						app.kubernetes.io/component: a-namespace-the-name-nonprivileged-cluster-binding
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: "a-namespace-the-name-nonprivileged-cluster-binding"
				subjects:
				-	kind: "ServiceAccount"
					name: "the-name"
					namespace: "a-namespace"
				roleRef:
					kind: "ClusterRole"
					name: "a-namespace-cluster-role-nonprivileged"
					apiGroup: "rbac.authorization.k8s.io"
			`, actualClusterRoleBinding)
		}

		actualClusterRole, err := RoundtripNode(clusterRole, config)
		if assert.NoError(t, err) {
			testhelpers.IsYAMLEqualString(assert.New(t), `---
				apiVersion: rbac.authorization.k8s.io/v1
				kind: ClusterRole
				metadata:
					labels:
						app.kubernetes.io/component: a-namespace-cluster-role-nonprivileged
						app.kubernetes.io/instance: MyRelease
						app.kubernetes.io/managed-by: Tiller
						app.kubernetes.io/name: MyChart
						app.kubernetes.io/version: 1.22.333.4444
						helm.sh/chart: MyChart-42.1_foo
						skiff-role-name: a-namespace-cluster-role-nonprivileged
					name: a-namespace-cluster-role-nonprivileged
				rules:
				-	apiGroups:     [policy]
					resourceNames: [psp-name]
					resources:     [podsecuritypolicies]
					verbs:         [use]
				-	apiGroups:     [imaginary]
					resourceNames: [unchanged]
					resources:     [other]
					verbs:         [yank]
			`, actualClusterRole)
		}
	})
}

func TestNewRBACRoleKube(t *testing.T) {
	t.Parallel()

	rbacRole, err := NewRBACRole("the-name",
		RBACRoleKindRole,
		[]model.AuthRule{
			{
				APIGroups: []string{"api-group-1"},
				Resources: []string{"resource-b"},
				Verbs:     []string{"verb-iii"},
			},
		},
		ExportSettings{})

	require.NoError(t, err)

	actual, err := RoundtripKube(rbacRole)
	require.NoError(t, err)
	testhelpers.IsYAMLEqualString(assert.New(t), `---
		apiVersion: "rbac.authorization.k8s.io/v1"
		kind: "Role"
		metadata:
			name: "the-name"
			labels:
				app.kubernetes.io/component: the-name
		rules:
		-	apiGroups:
			-	"api-group-1"
			resources:
			-	"resource-b"
			verbs:
			-	"verb-iii"
	`, actual)
}

func TestNewRBACRoleHelm(t *testing.T) {
	t.Parallel()

	rbacRole, err := NewRBACRole("the-name",
		RBACRoleKindRole,
		[]model.AuthRule{
			{
				APIGroups: []string{"api-group-1"},
				Resources: []string{"resource-b"},
				Verbs:     []string{"verb-iii"},
			},
		},
		ExportSettings{
			CreateHelmChart: true,
		})

	require.NoError(t, err)

	t.Run("NoAuth", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth": "",
		}

		actual, err := RoundtripNode(rbacRole, config)
		require.NoError(t, err)

		testhelpers.IsYAMLEqualString(assert.New(t), `---
		`, actual)
	})

	t.Run("HasAuth", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth": "rbac",
		}

		actual, err := RoundtripNode(rbacRole, config)
		require.NoError(t, err)

		testhelpers.IsYAMLEqualString(assert.New(t), `---
			apiVersion: "rbac.authorization.k8s.io/v1"
			kind: "Role"
			metadata:
				name: "the-name"
				labels:
					app.kubernetes.io/component: the-name
					app.kubernetes.io/instance: MyRelease
					app.kubernetes.io/managed-by: Tiller
					app.kubernetes.io/name: MyChart
					app.kubernetes.io/version: 1.22.333.4444
					helm.sh/chart: MyChart-42.1_foo
					skiff-role-name: "the-name"
			rules:
			-	apiGroups:
				-	"api-group-1"
				resources:
				-	"resource-b"
				verbs:
				-	"verb-iii"
		`, actual)
	})
}

/*
func TestNewRBACClusterRolePSPKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	resource, err := NewRBACClusterRolePSP("the-name",
		ExportSettings{})

	if !assert.NoError(err) {
		return
	}

	actualCR, err := RoundtripKube(resource)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "rbac.authorization.k8s.io/v1"
		kind: "ClusterRole"
		metadata:
			name: "psp-role-the-name"
			labels:
				app.kubernetes.io/component: psp-role-the-name
		rules:
		-	apiGroups:
			-	"extensions"
			resourceNames:
			-	"the-name"
			resources:
			-	"podsecuritypolicies"
			verbs:
			-	"use"
	`, actualCR)
}

func TestNewRBACClusterRolePSPHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	resource, err := NewRBACClusterRolePSP("the_name",
		ExportSettings{
			CreateHelmChart: true,
		})
	if !assert.NoError(err) {
		return
	}

	config := map[string]interface{}{
		"Values.kube.auth":         "rbac",
		"Values.kube.psp.the_name": "foo",
		"Release.Namespace":        "namespace",
	}
	actualCR, err := RoundtripNode(resource, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "rbac.authorization.k8s.io/v1"
		kind: "ClusterRole"
		metadata:
			name: "namespace-psp-role-the_name"
			labels:
				app.kubernetes.io/component: namespace-psp-role-the_name
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "namespace-psp-role-the_name"
		rules:
		-	apiGroups:
			-	"extensions"
			resourceNames:
			-	"foo"
			resources:
			-	"podsecuritypolicies"
			verbs:
			-	"use"
	`, actualCR)
}
*/
