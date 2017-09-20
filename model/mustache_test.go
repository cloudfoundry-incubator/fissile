package model

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	retCode := m.Run()

	os.Exit(retCode)
}

func TestParsing(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	template := "((FISSILE_IDENTITY_SCHEME))://((#FISSILE_IDENTITY_EXTERNAL_HOST))((FISSILE_INSTANCE_ID)).((FISSILE_IDENTITY_EXTERNAL_HOST)):((FISSILE_IDENTITY_EXTERNAL_PORT))((/FISSILE_IDENTITY_EXTERNAL_HOST))((^FISSILE_IDENTITY_EXTERNAL_HOST))scf.uaa-int.((FISSILE_SERVICE_DOMAIN_SUFFIX)):8443((/FISSILE_IDENTITY_EXTERNAL_HOST))"

	// Act
	pieces, err := parseTemplate(template)

	// Assert
	assert.NoError(err)
	assert.Contains(pieces, "FISSILE_INSTANCE_ID")
	assert.Contains(pieces, "FISSILE_IDENTITY_EXTERNAL_HOST")
	assert.Contains(pieces, "FISSILE_IDENTITY_EXTERNAL_PORT")
	assert.Contains(pieces, "FISSILE_SERVICE_DOMAIN_SUFFIX")
	assert.NotContains(pieces, "FOO")
}

func TestRoleVariables(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
	assert.NoError(err)
	assert.NotNil(roleManifest)

	vars, err := roleManifest.Roles[0].GetVariablesForRole()

	assert.NoError(err)
	assert.NotNil(vars)

	expected := []string{"HOME", "FOO", "BAR", "KUBE_SERVICE_DOMAIN_SUFFIX", "PELERINUL"}
	sort.Strings(expected)
	var actual []string
	for _, variable := range vars {
		actual = append(actual, variable.Name)
	}
	sort.Strings(actual)
	assert.Equal(expected, actual)
}
