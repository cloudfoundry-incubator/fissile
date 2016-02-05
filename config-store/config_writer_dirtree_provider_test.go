package configstore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/model"

	"github.com/stretchr/testify/assert"
)

func TestConfigStoreDirTreeWriter(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	tmpDir, err := ioutil.TempDir("", "fissile-config-dirtree-tests")
	assert.Nil(err)
	defer os.RemoveAll(tmpDir)
	outDir := filepath.Join(tmpDir, "store")

	confStore := NewConfigStoreBuilder("foo", DirTreeProvider, opinionsFile, opinionsFileDark, outDir)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	err = confStore.WriteBaseConfig(rolesManifest.Roles)

	assert.Nil(err)

	descriptionValuePath := filepath.Join(outDir, "foo", "descriptions", "tor", "private_key", leafFilename)
	buf, err := ioutil.ReadFile(descriptionValuePath)
	assert.Nil(err)
	assert.Equal(string(buf), "The private key for this hidden service.\n")
}
