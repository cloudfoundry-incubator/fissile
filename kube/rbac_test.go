package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"
)

func TestNewRBACAccountKube(t *testing.T) {
	assert := assert.New(t)

	resources, err := NewRBACAccount("the-name",
		model.AuthAccount{
			Roles: []string{"a-role"},
		}, ExportSettings{})

	if !assert.NoError(err) {
		return
	}
	if !assert.Len(resources, 2, "Should have account plus role") {
		return
	}

	rbacAccount := resources[0]
	actualAccount, err := testhelpers.RoundtripKube(rbacAccount)
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
	actualRole, err := testhelpers.RoundtripKube(rbacRole)
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
}

func TestNewRBACAccountHelmNoAuth(t *testing.T) {
	assert := assert.New(t)

	resources, err := NewRBACAccount("the-name",
		model.AuthAccount{
			Roles: []string{"a-role"},
		}, ExportSettings{
			CreateHelmChart: true,
		})

	if !assert.NoError(err) {
		return
	}
	if !assert.Len(resources, 2, "Should have account plus role") {
		return
	}

	rbacAccount := resources[0]
	rbacRole := resources[1]

	t.Run("NoAuth", func(t *testing.T) {
		// config: .Values.kube.auth -- helm only ("", "rbac")
		actualAccount, err := testhelpers.RoundtripNode(rbacAccount, nil)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
		`, actualAccount)

		// config: .Values.kube.auth helm only ("", "rbac")
		actualRole, err := testhelpers.RoundtripNode(rbacRole, nil)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
		`, actualRole)
	})

	t.Run("HasAuth", func(t *testing.T) {
		config := map[string]interface{}{
			"Values.kube.auth": "rbac",
		}

		actualAccount, err := testhelpers.RoundtripNode(rbacAccount, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "ServiceAccount"
			metadata:
				name: "the-name"
		`, actualAccount)

		actualRole, err := testhelpers.RoundtripNode(rbacRole, config)
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
}

func TestNewRBACRoleKube(t *testing.T) {
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

	actual, err := testhelpers.RoundtripKube(rbacRole)
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
		// config: .Values.kube.auth helm only ("", "rbac")
		actual, err := testhelpers.RoundtripNode(rbacRole, nil)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
		`, actual)
	})

	t.Run("HasAuth", func(t *testing.T) {
		config := map[string]interface{}{
			"Values.kube.auth": "rbac",
		}

		actual, err := testhelpers.RoundtripNode(rbacRole, config)
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
