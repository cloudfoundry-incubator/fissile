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
	assert.NoError(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	tmpDir, err := ioutil.TempDir("", "fissile-config-dirtree-tests")
	assert.NoError(err)
	defer os.RemoveAll(tmpDir)
	outDir := filepath.Join(tmpDir, "store")

	confStore := NewConfigStoreBuilder("foo", DirTreeProvider, opinionsFile, opinionsFileDark, outDir)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := model.NewRelease(releasePath)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	err = confStore.WriteBaseConfig(rolesManifest)

	assert.NoError(err)

	descriptionValuePath := filepath.Join(outDir, "foo", "descriptions", "tor", "private_key", leafFilename)
	buf, err := ioutil.ReadFile(descriptionValuePath)
	assert.NoError(err)
	assert.Equal(string(buf), "The private key for this hidden service.\n")
}
