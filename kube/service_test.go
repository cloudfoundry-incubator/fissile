package kube

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
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
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release})
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
	service, err := NewClusterIPService(role, false, false, &ExportSettings{})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Equal("ClusterIP", service.Get("spec/type").Value())
	assert.Nil(service.Get("spec/clusterIP"))

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(service); !assert.NoError(err) {
		return
	}
	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}
	expectedYAML := strings.Replace(`---
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
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	testhelpers.IsYAMLSubset(assert, expected, actual)
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
	service, err := NewClusterIPService(role, true, false, &ExportSettings{})
	if !assert.NoError(err) {
		return
	}
	require.NotNil(t, service)
	assert.Equal("ClusterIP", service.Get("spec/type").Value())
	assert.Equal("None", service.Get("spec/clusterIP").Value())

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(service); !assert.NoError(err) {
		return
	}
	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}
	expectedYAML := strings.Replace(`---
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
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	testhelpers.IsYAMLSubset(assert, expected, actual)
}
