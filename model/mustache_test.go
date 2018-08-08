package model

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	retCode := m.Run()

	os.Exit(retCode)
}

func TestParsing(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	template := "((FISSILE_IDENTITY_SCHEME))://((#FISSILE_IDENTITY_EXTERNAL_HOST))((FISSILE_INSTANCE_ID)).((FISSILE_IDENTITY_EXTERNAL_HOST)):((FISSILE_IDENTITY_EXTERNAL_PORT))((/FISSILE_IDENTITY_EXTERNAL_HOST))((^FISSILE_IDENTITY_EXTERNAL_HOST))scf.uaa-int.uaa.svc.((FISSILE_CLUSTER_DOMAIN)):8443((/FISSILE_IDENTITY_EXTERNAL_HOST))"

	// Act
	pieces, err := parseTemplate(template)

	// Assert
	assert.NoError(err)
	assert.Contains(pieces, "FISSILE_INSTANCE_ID")
	assert.Contains(pieces, "FISSILE_IDENTITY_EXTERNAL_HOST")
	assert.Contains(pieces, "FISSILE_IDENTITY_EXTERNAL_PORT")
	assert.Contains(pieces, "FISSILE_CLUSTER_DOMAIN")
	assert.NotContains(pieces, "FOO")
}

func TestRoleVariables(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(t, err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/model/variable-expansion.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	vars, err := roleManifest.InstanceGroups[0].GetVariablesForRole()

	assert.NoError(t, err)
	assert.NotNil(t, vars)

	expected := []string{"HOME", "FOO", "BAR", "KUBERNETES_CLUSTER_DOMAIN", "PELERINUL"}
	sort.Strings(expected)
	var actual []string
	for _, variable := range vars {
		actual = append(actual, variable.Name)
	}
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
}
