package resolver_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetScriptPaths(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/tor-good.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	fullScripts := roleManifest.InstanceGroups[0].GetScriptPaths()
	assert.Len(t, fullScripts, 3)
	for _, leafName := range []string{"environ.sh", "myrole.sh", "post_config_script.sh"} {
		assert.Equal(t, filepath.Join(workDir, "../../test-assets/role-manifests/model/scripts", leafName), fullScripts["scripts/"+leafName])
	}
}

func TestRoleVariables(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/variable-expansion.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	vars, err := roleManifest.InstanceGroups[0].GetVariablesForRole()

	assert.NoError(t, err)
	assert.NotNil(t, vars)

	expected := []string{"HOME", "FOO", "BAR", "KUBERNETES_CLUSTER_DOMAIN", "KUBERNETES_CONTAINER_NAME", "PELERINUL"}
	sort.Strings(expected)
	var actual []string
	for _, variable := range vars {
		actual = append(actual, variable.Name)
	}
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
}
