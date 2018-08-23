package compilator

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/SUSE/fissile/docker"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/scripts/compilation"
	"github.com/SUSE/fissile/util"

	"github.com/graymeta/stow"
	"github.com/graymeta/stow/local"
	"github.com/stretchr/testify/assert"
)

func TestStorePackageLocallyOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.NoError(err)
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.NoError(err)
	imageName := "splatform/fissile-stemcell-opensuse:42.2"

	workDir, err := os.Getwd()
	assert.NoError(err)

	packageCacheConfigFilename := filepath.Join(workDir, "../test-assets/package-cache-config/localexample.json")
	packageCacheConfigReader, err := ioutil.ReadFile(packageCacheConfigFilename)
	assert.NoError(err)

	var packageCacheConfig map[string]interface{}

	err = yaml.Unmarshal(packageCacheConfigReader, &packageCacheConfig)
	assert.NoError(err)

	var configMap stow.ConfigMap
	configMap = make(stow.ConfigMap)

	for key, value := range packageCacheConfig {
		configMap.Set(key, value.(string))
	}
	containerLocation, err := util.TempDir("", "fissile-stow-tests")
	defer os.RemoveAll(containerLocation)

	p, err := NewPackageStorage(packageCacheConfig["kind"].(string), false, configMap, compilationWorkDir, containerLocation, imageName)
	assert.NoError(err)

	localKeyPath, _ := p.Config.Config(local.ConfigKeyPath)
	fullContainerPath := filepath.Join(localKeyPath, containerLocation)
	defer os.RemoveAll(fullContainerPath)

	c, err := NewDockerCompilator(dockerManager, compilationWorkDir, "", imageName, compilation.FakeBase, "3.14.15", "", false, ui, nil, p)
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	// Act
	err = c.compilePackageInDocker(release.Packages[0])
	// Upload (stow)
	pack := release.Packages[0]

	err = p.Upload(pack)
	assert.NoError(err)

	err = p.Download(pack)
	assert.NoError(err)

	exists, err := p.Exists(pack)
	assert.NoError(err)

	//Assert
	assert.True(exists)
	assert.NoError(err)
}

func TestStorePackageExists(t *testing.T) {
	//Arrange
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.NoError(err)
	imageName := "splatform/fissile-stemcell-opensuse:42.2"

	workDir, err := os.Getwd()
	assert.NoError(err)

	packageCacheConfigFilename := filepath.Join(workDir, "../test-assets/package-cache-config/localexample.json")
	packageCacheConfigReader, err := ioutil.ReadFile(packageCacheConfigFilename)
	assert.NoError(err)

	var packageCacheConfig map[string]interface{}

	err = yaml.Unmarshal(packageCacheConfigReader, &packageCacheConfig)
	assert.NoError(err)

	var configMap stow.ConfigMap
	configMap = make(stow.ConfigMap)

	for key, value := range packageCacheConfig {
		configMap.Set(key, value.(string))
	}
	containerLocation, err := util.TempDir("", "fissile-stow-tests")
	defer os.RemoveAll(containerLocation)
	p, err := NewPackageStorage(packageCacheConfig["kind"].(string), false, configMap, compilationWorkDir, containerLocation, imageName)
	assert.NoError(err)

	localKeyPath, _ := p.Config.Config(local.ConfigKeyPath)
	fullContainerPath := filepath.Join(localKeyPath, containerLocation)
	defer os.RemoveAll(fullContainerPath)

	c, err := NewDockerCompilator(dockerManager, compilationWorkDir, "", imageName, compilation.FakeBase, "3.14.15", "", false, ui, nil, p)
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	//Act
	err = c.compilePackageInDocker(release.Packages[0])

	existsFalse, err := p.Exists(release.Packages[0])
	assert.NoError(err)

	p.Upload(release.Packages[0])

	existsTrue, err := p.Exists(release.Packages[0])
	assert.NoError(err)

	//Assert
	assert.False(existsFalse)
	assert.True(existsTrue)
}
