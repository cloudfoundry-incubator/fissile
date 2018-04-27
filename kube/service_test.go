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

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests", manifestName)
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

func TestServiceOK(t *testing.T) {
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
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Equal("ClusterIP", service.Get("spec", "type").String())
	assert.Nil(service.Get("spec", "clusterIP"))

	actual, err := testhelpers.RoundtripNode(service, nil)
	if !assert.NoError(err) {
		return
	}
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

func TestServiceHelmOKDefaults(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, false, false, ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Nil(service.Get("spec", "clusterIP"))

	actual, err := testhelpers.RoundtripNode(service, nil)
	if !assert.NoError(err) {
		return
	}
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
}

func TestServiceHelmOKConfigured(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, false, false, ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Nil(service.Get("spec", "clusterIP"))

	config := map[string]interface{}{
		"Values.services.loadbalanced": "true",
	}

	actual, err := testhelpers.RoundtripNode(service, config)
	if !assert.NoError(err) {
		return
	}
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
}

func TestHeadlessServiceOK(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, true, false, ExportSettings{})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Equal("ClusterIP", service.Get("spec", "type").String())
	assert.Equal("None", service.Get("spec", "clusterIP").String())

	actual, err := testhelpers.RoundtripNode(service, nil)
	if !assert.NoError(err) {
		return
	}
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

func TestHeadlessServiceHelmOKDefaults(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, true, false, ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Equal("None", service.Get("spec", "clusterIP").String())

	actual, err := testhelpers.RoundtripNode(service, nil)
	if !assert.NoError(err) {
		return
	}
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
}

func TestHeadlessServiceHelmOKConfigured(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, true, false, ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Equal("None", service.Get("spec", "clusterIP").String())

	config := map[string]interface{}{
		"Values.services.loadbalanced": "true",
	}

	actual, err := testhelpers.RoundtripNode(service, config)
	if !assert.NoError(err) {
		return
	}
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
}

func TestPublicServiceOK(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, false, true, ExportSettings{})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Equal("ClusterIP", service.Get("spec", "type").String())
	assert.Nil(service.Get("spec", "clusterIP"))

	actual, err := testhelpers.RoundtripNode(service, nil)
	if !assert.NoError(err) {
		return
	}
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

func TestPublicServiceHelmOKDefaults(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, false, true, ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)

	// Well, not quite just defaults. Provide the one piece of
	// information required by the template to not fail rendering.
	config := map[string]interface{}{
		"Values.kube.external_ips": "[127.0.0.1,127.0.0.2]",
	}

	actual, err := testhelpers.RoundtripNode(service, config)
	if !assert.NoError(err) {
		return
	}
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
}

func TestPublicServiceHelmOKConfigured(t *testing.T) {
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := NewClusterIPService(role, false, true, ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)

	config := map[string]interface{}{
		"Values.services.loadbalanced": "true",
		"Values.kube.external_ips":     "",
	}

	actual, err := testhelpers.RoundtripNode(service, config)
	if !assert.NoError(err) {
		return
	}
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
}
