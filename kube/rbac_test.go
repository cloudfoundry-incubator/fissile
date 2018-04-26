package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"
)

func TestNewRBACAccountForKube(t *testing.T) {
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
	actual, err := testhelpers.RoundtripNode(rbacAccount, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "ServiceAccount"
		metadata:
			name: "the-name"
	`, actual)

	rbacRole := resources[1]
	// config: .Values.kube.auth -- helm only ("", "rbac")
	actual, err = testhelpers.RoundtripNode(rbacRole, nil)
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
	`, actual)
}

func TestNewRBACAccountForHelmNoAuth(t *testing.T) {
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
	// config: .Values.kube.auth -- helm only ("", "rbac")
	actual, err := testhelpers.RoundtripNode(rbacAccount, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
	`, actual)

	rbacRole := resources[1]
	// config: .Values.kube.auth helm only ("", "rbac")
	actual, err = testhelpers.RoundtripNode(rbacRole, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
	`, actual)
}

func TestNewRBACAccountForHelmWithAuth(t *testing.T) {
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

	authConfig := map[string]interface{}{
		"Values.kube.auth": "rbac",
	}

	rbacAccount := resources[0]
	actual, err := testhelpers.RoundtripNode(rbacAccount, authConfig)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "ServiceAccount"
		metadata:
			name: "the-name"
	`, actual)

	rbacRole := resources[1]
	actual, err = testhelpers.RoundtripNode(rbacRole, authConfig)
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
	`, actual)
}

func TestNewRBACRoleForKube(t *testing.T) {
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

	// config: .Values.kube.auth true/false - Helm only
	actual, err := testhelpers.RoundtripNode(rbacRole, nil)
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

func TestNewRBACRoleForHelmNoAuth(t *testing.T) {
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

	// config: .Values.kube.auth helm only ("", "rbac")
	actual, err := testhelpers.RoundtripNode(rbacRole, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
	`, actual)
}

func TestNewRBACRoleForHelmWithAuth(t *testing.T) {
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
}
