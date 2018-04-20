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
	accountYAML, err := testhelpers.RenderNode(rbacAccount, nil)

	if !assert.NoError(err) {
		return
	}

	expectedAccountYAML := `---
apiVersion: "v1"
kind: "ServiceAccount"
metadata:
  name: "the-name"
`
	assert.Equal(expectedAccountYAML, string(accountYAML))

	rbacRole := resources[1]
	roleYAML, err := testhelpers.RenderNode(rbacRole, nil)
	// config: .Values.kube.auth helm only ("", "rbac")

	if !assert.NoError(err) {
		return
	}

	expectedRoleYAML := `---
apiVersion: "rbac.authorization.k8s.io/v1beta1"
kind: "RoleBinding"
metadata:
  name: "the-name-a-role-binding"
subjects:
- kind: "ServiceAccount"
  name: "the-name"
roleRef:
  kind: "Role"
  name: "a-role"
  apiGroup: "rbac.authorization.k8s.io"
`
	assert.Equal(expectedRoleYAML, string(roleYAML))
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
	accountYAML, err := testhelpers.RenderNode(rbacAccount, nil)
	// config: .Values.kube.auth helm only ("", "rbac")

	if !assert.NoError(err) {
		return
	}

	expectedAccountYAML := `---
`
	assert.Equal(expectedAccountYAML, string(accountYAML))

	rbacRole := resources[1]
	roleYAML, err := testhelpers.RenderNode(rbacRole, nil)
	// config: .Values.kube.auth helm only ("", "rbac")

	if !assert.NoError(err) {
		return
	}

	expectedRoleYAML := `---
`
	assert.Equal(expectedRoleYAML, string(roleYAML))
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
	accountYAML, err := testhelpers.RenderNode(rbacAccount, authConfig)

	if !assert.NoError(err) {
		return
	}

	expectedAccountYAML := `---
apiVersion: "v1"
kind: "ServiceAccount"
metadata:
  name: "the-name"
`
	assert.Equal(expectedAccountYAML, string(accountYAML))

	rbacRole := resources[1]
	roleYAML, err := testhelpers.RenderNode(rbacRole, authConfig)

	if !assert.NoError(err) {
		return
	}

	expectedRoleYAML := `---
apiVersion: "rbac.authorization.k8s.io/v1beta1"
kind: "RoleBinding"
metadata:
  name: "the-name-a-role-binding"
subjects:
- kind: "ServiceAccount"
  name: "the-name"
roleRef:
  kind: "Role"
  name: "a-role"
  apiGroup: "rbac.authorization.k8s.io"
`
	assert.Equal(expectedRoleYAML, string(roleYAML))
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

	roleYAML, err := testhelpers.RenderNode(rbacRole, nil)
	// config: .Values.kube.auth true/false - Helm only

	if !assert.NoError(err) {
		return
	}

	expectedYAML := `---
apiVersion: "rbac.authorization.k8s.io/v1beta1"
kind: "Role"
metadata:
  name: "the-name"
rules:
- apiGroups:
  - "api-group-1"
  resources:
  - "resource-b"
  verbs:
  - "verb-iii"
`
	assert.Equal(expectedYAML, string(roleYAML))
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

	roleYAML, err := testhelpers.RenderNode(rbacRole, nil)
	// config: .Values.kube.auth helm only ("", "rbac")

	if !assert.NoError(err) {
		return
	}

	expectedYAML := `---
`
	assert.Equal(expectedYAML, string(roleYAML))
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

	roleYAML, err := testhelpers.RenderNode(rbacRole,
		map[string]interface{}{
			"Values.kube.auth": "rbac",
		})

	if !assert.NoError(err) {
		return
	}

	expectedYAML := `---
apiVersion: "rbac.authorization.k8s.io/v1beta1"
kind: "Role"
metadata:
  name: "the-name"
rules:
- apiGroups:
  - "api-group-1"
  resources:
  - "resource-b"
  verbs:
  - "verb-iii"
`
	assert.Equal(expectedYAML, string(roleYAML))
}
