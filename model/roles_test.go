package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadRoleManifestOK(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := LoadRoleManifest(roleManifestPath, release)
	assert.Nil(err)
	assert.NotNil(rolesManifest)

	assert.Equal(2, len(rolesManifest.Roles))
	assert.Equal("tor", rolesManifest.Roles[1].Jobs[0].Name)
	assert.NotNil(rolesManifest.Roles[1].Jobs[0].Release)
	assert.Equal("tor", rolesManifest.Roles[1].Jobs[0].Release.Name)
}

func TestLoadRoleManifestNotOKBadJobName(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-bad.yml")
	_, err = LoadRoleManifest(roleManifestPath, release)
	assert.NotNil(err)
	assert.Contains(err.Error(), "Cannot find job foo in release")
}
