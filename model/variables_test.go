package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDeploymentManifestVariables(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(t, err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/deployment-manifests/bosh-deployment.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	assert.Len(t, roleManifest.Variables, 2)
	assert.Equal(t, "admin_password", roleManifest.Variables[0].Name)
	assert.Equal(t, true, roleManifest.Variables[1].CVOptions.Secret)
}
