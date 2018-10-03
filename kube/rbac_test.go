package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/testhelpers"
)

func TestNewRBACAccountPSPKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	resources, err := NewRBACAccount("the-name",
		model.AuthAccount{
			Roles:             []string{"a-role"},
			PodSecurityPolicy: "privileged",
		}, ExportSettings{})

	if !assert.NoError(err) {
		return
	}
	if !assert.Len(resources, 3, "Should have account, role and cluster role bindings") {
		return
	}

	rbacAccount := resources[0]
	actualAccount, err := RoundtripKube(rbacAccount)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "ServiceAccount"
		metadata:
			name: "the-name"
	`, actualAccount)

	rbacRole := resources[1]
	actualRole, err := RoundtripKube(rbacRole)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "rbac.authorization.k8s.io/v1beta1"
		kind: "RoleBinding"
		metadata:
			name: "the-name-a-role-binding"
		subjects:
		-	kind: "ServiceAccount"
			name: "the-name"
		roleRef:
			kind: "Role"
			name: "a-role"
			apiGroup: "rbac.authorization.k8s.io"
	`, actualRole)

	pspBinding := resources[2]
	actualBinding, err := RoundtripKube(pspBinding)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "rbac.authorization.k8s.io/v1"
		kind: "ClusterRoleBinding"
		metadata:
			name: "the-name-binding-psp"
		subjects:
		-	kind: "ServiceAccount"
			name: "the-name"
			namespace: "~"
		roleRef:
			kind: "ClusterRole"
			name: "psp-role-privileged"
			apiGroup: "rbac.authorization.k8s.io"
	`, actualBinding)
}

func TestNewRBACAccountHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	resources, err := NewRBACAccount("the-name",
		model.AuthAccount{
			Roles:             []string{"a-role"},
			PodSecurityPolicy: "nonprivileged",
		}, ExportSettings{
			CreateHelmChart: true,
		})

	if !assert.NoError(err) {
		return
	}
	if !assert.Len(resources, 3, "Should have account plus role and cluster role bindings") {
		return
	}

	rbacAccount := resources[0]
	rbacRole := resources[1]
	pspBinding := resources[2]

	t.Run("NoAuth", func(t *testing.T) {
		t.Parallel()
		// config: .Values.kube.auth -- helm only ("", "rbac")
		actualAccount, err := RoundtripNode(rbacAccount, nil)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
		`, actualAccount)

		// config: .Values.kube.auth helm only ("", "rbac")
		actualRole, err := RoundtripNode(rbacRole, nil)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
		`, actualRole)
	})

	t.Run("HasAuth", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth": "rbac",
		}

		actualAccount, err := RoundtripNode(rbacAccount, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "ServiceAccount"
			metadata:
				name: "the-name"
		`, actualAccount)

		actualRole, err := RoundtripNode(rbacRole, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "rbac.authorization.k8s.io/v1beta1"
			kind: "RoleBinding"
			metadata:
				name: "the-name-a-role-binding"
			subjects:
			-	kind: "ServiceAccount"
				name: "the-name"
			roleRef:
				kind: "Role"
				name: "a-role"
				apiGroup: "rbac.authorization.k8s.io"
		`, actualRole)
	})

	t.Run("NoPodSecurityPolicy", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth": "rbac",
		}
		// config: .Values.kube.psp.nonprivileged: ~
		actualAccount, err := RoundtripNode(rbacAccount, config)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "ServiceAccount"
			metadata:
				name: "the-name"
		`, actualAccount)

		// config: .Values.kube.psp.nonprivileged: ~
		actualBinding, err := RoundtripNode(pspBinding, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
		`, actualBinding)
	})

	t.Run("HasPodSecurityPolicy", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth":              "rbac",
			"Values.kube.psp.nonprivileged": "foo",
			"Release.Namespace":             "a-namespace",
		}

		actualAccount, err := RoundtripNode(rbacAccount, config)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "ServiceAccount"
			metadata:
				name: "the-name"
		`, actualAccount)

		actualBinding, err := RoundtripNode(pspBinding, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "rbac.authorization.k8s.io/v1"
			kind: "ClusterRoleBinding"
			metadata:
				name: "a-namespace-the-name-binding-psp"
			subjects:
			-	kind: "ServiceAccount"
				name: "the-name"
				namespace: "a-namespace"
			roleRef:
				kind: "ClusterRole"
				name: "a-namespace-psp-role-nonprivileged"
				apiGroup: "rbac.authorization.k8s.io"
		`, actualBinding)
	})
}

func TestNewRBACRoleKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	rbacRole, err := NewRBACRole("the-name",
		[]model.AuthRule{
			{
				APIGroups: []string{"api-group-1"},
				Resources: []string{"resource-b"},
				Verbs:     []string{"verb-iii"},
			},
		},
		ExportSettings{})

	if !assert.NoError(err) {
		return
	}

	actual, err := RoundtripKube(rbacRole)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "rbac.authorization.k8s.io/v1beta1"
		kind: "Role"
		metadata:
			name: "the-name"
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
	assert := assert.New(t)

	rbacRole, err := NewRBACRole("the-name",
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

	if !assert.NoError(err) {
		return
	}

	t.Run("NoAuth", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth": "",
		}

		actual, err := RoundtripNode(rbacRole, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
		`, actual)
	})

	t.Run("HasAuth", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.auth": "rbac",
		}

		actual, err := RoundtripNode(rbacRole, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "rbac.authorization.k8s.io/v1beta1"
			kind: "Role"
			metadata:
				name: "the-name"
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
