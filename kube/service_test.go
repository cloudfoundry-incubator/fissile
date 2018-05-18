package kube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serviceTestLoadRole(assert *assert.Assertions, manifestName string) (*model.RoleManifest, *model.Role) {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/kube", manifestName)
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")

	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return nil, nil
	}
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release}, nil)
	if !assert.NoError(err) {
		return nil, nil
	}
	role := manifest.LookupRole("myrole")
	if !assert.NotNil(role, "Failed to find role in manifest") {
		return nil, nil
	}
	return manifest, role
}

func TestServiceKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, false, false, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := testhelpers.RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole
		spec:
			ports:
			-
				name: http
				port: 80
				targetPort: http
			-
				name: https
				port: 443
				targetPort: https
			selector:
				skiff-role-name: myrole
			type: ClusterIP
	`, actual)
}

func TestServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)
	service, err := NewClusterIPService(role, false, false, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		actual, err := testhelpers.RoundtripNode(service, nil)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole"
			spec:
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: "http"
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
				selector:
					skiff-role-name: "myrole"
				type:	ClusterIP
		`, actual)
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
		}

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole"
			spec:
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: "http"
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
				selector:
					skiff-role-name: "myrole"
				type:	LoadBalancer
		`, actual)
	})
}

func TestHeadlessServiceKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := NewClusterIPService(role, true, false, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := testhelpers.RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole-set
		spec:
			ports:
			-
				name: http
				port: 80
				# targetPort must be undefined for headless services
				targetPort: 0
			-
				name: https
				port: 443
				# targetPort must be undefined for headless services
				targetPort: 0
			selector:
				skiff-role-name: myrole
			type: ClusterIP
			clusterIP: None
	`, actual)
}

func TestHeadlessServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := NewClusterIPService(role, true, false, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		actual, err := testhelpers.RoundtripNode(service, nil)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-set"
			spec:
				clusterIP: "None"
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: 0
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 0
				selector:
					skiff-role-name: "myrole"
				type:	ClusterIP
		`, actual)
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
		}

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-set"
			spec:
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: 0
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 0
				selector:
					skiff-role-name: "myrole"
				type:	LoadBalancer
		`, actual)
	})
}

func TestPublicServiceKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := NewClusterIPService(role, false, true, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := testhelpers.RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole-public
		spec:
			externalIPs: '[ 192.168.77.77 ]'
			ports:
			-
				name: https
				port: 443
				targetPort: https
			selector:
				skiff-role-name: myrole
			type: ClusterIP
	`, actual)
}

func TestPublicServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := NewClusterIPService(role, false, true, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.external_ips": "[127.0.0.1,127.0.0.2]",
		}

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-public"
			spec:
				externalIPs: "[127.0.0.1,127.0.0.2]"
				ports:
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
				selector:
					skiff-role-name: "myrole"
				type:	ClusterIP
		`, actual)
	})

	t.Run("LoadBalanced", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
			"Values.kube.external_ips":     "",
		}

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-public"
			spec:
				externalIPs: ""
				ports:
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
				selector:
					skiff-role-name: "myrole"
				type:	LoadBalancer
		`, actual)
	})
}
