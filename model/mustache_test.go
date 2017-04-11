package model

import (
	"os"
	"path/filepath"
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
	template := "((HCP_IDENTITY_SCHEME))://((#HCP_IDENTITY_EXTERNAL_HOST))((HCP_INSTANCE_ID)).((HCP_IDENTITY_EXTERNAL_HOST)):((HCP_IDENTITY_EXTERNAL_PORT))((/HCP_IDENTITY_EXTERNAL_HOST))((^HCP_IDENTITY_EXTERNAL_HOST))hcf.uaa-int.((HCP_SERVICE_DOMAIN_SUFFIX)):8443((/HCP_IDENTITY_EXTERNAL_HOST))"

	// Act
	pieces, err := parseTemplate(template)

	// Assert
	assert.NoError(err)
	assert.Contains(pieces, "HCP_INSTANCE_ID")
	assert.Contains(pieces, "HCP_IDENTITY_EXTERNAL_HOST")
	assert.Contains(pieces, "HCP_IDENTITY_EXTERNAL_PORT")
	assert.Contains(pieces, "HCP_SERVICE_DOMAIN_SUFFIX")
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
	rolesManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
	assert.NoError(err)
	assert.NotNil(rolesManifest)

	vars, err := rolesManifest.Roles[0].GetVariablesForRole()

	assert.NoError(err)
	assert.NotNil(vars)
	assert.Len(vars, 4)
	assert.Contains([]string{"HOME", "FOO", "BAR", "PELERINUL"}, vars[0].Name)
	assert.Contains([]string{"HOME", "FOO", "BAR", "PELERINUL"}, vars[1].Name)
	assert.Contains([]string{"HOME", "FOO", "BAR", "PELERINUL"}, vars[2].Name)
	assert.Contains([]string{"HOME", "FOO", "BAR", "PELERINUL"}, vars[3].Name)
}
