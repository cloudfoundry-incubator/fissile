package compilator

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"

	"code.cloudfoundry.org/fissile/docker"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/scripts/compilation"
	"code.cloudfoundry.org/fissile/util"

	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
	"github.com/graymeta/stow"
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

	packageCacheConfigFilename := filepath.Join(workDir, "../test-assets/package-cache-config/localexample.yaml")
	packageCacheConfigReader, err := ioutil.ReadFile(packageCacheConfigFilename)
	assert.NoError(err)

	var packageCacheConfig map[string]interface{}

	err = yaml.Unmarshal(packageCacheConfigReader, &packageCacheConfig)
	assert.NoError(err)

	var configMap stow.ConfigMap
	configMap = make(stow.ConfigMap)

	for key, value := range packageCacheConfig {
		if key != "boshPackageCacheKind" && key != "boshPackageCacheReadOnly" && key != "boshPackageCacheLocation" {
			configMap.Set(key, value.(string))
		}
	}
	containerDir, err := util.TempDir("", "fissile-stow-tests")
	fullContainerPath := filepath.Join(containerDir, "cache")
	defer os.RemoveAll(containerDir)
	p, err := NewPackageStorage(packageCacheConfig["boshPackageCacheKind"].(string), false, configMap, compilationWorkDir, fullContainerPath, imageName)
	assert.NoError(err)

	c, err := NewDockerCompilator(dockerManager, compilationWorkDir, "", imageName, compilation.FakeBase, "3.14.15", "", false, ui, nil, p)
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathBoshCache := filepath.Join(workDir, "../test-assets/bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	// Act
	err = c.compilePackageInDocker(release.Packages[0])
	// Upload (stow)
	pack := release.Packages[0]

	err = p.Upload(pack)
	assert.NoError(err)

	bar := uiprogress.AddBar(100)
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.Resize(fmt.Sprintf("cache: %s/%s", pack.Release.Name, pack.Name), 22)
	})

	err = p.Download(pack, func(progress float64) {
		if progress == -1 {
			return
		}

		bar.Set(int(progress))
	})
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

	packageCacheConfigFilename := filepath.Join(workDir, "../test-assets/package-cache-config/localexample.yaml")
	packageCacheConfigReader, err := ioutil.ReadFile(packageCacheConfigFilename)
	assert.NoError(err)

	var packageCacheConfig map[string]interface{}

	err = yaml.Unmarshal(packageCacheConfigReader, &packageCacheConfig)
	assert.NoError(err)

	var configMap stow.ConfigMap
	configMap = make(stow.ConfigMap)

	for key, value := range packageCacheConfig {
		if key != "boshPackageCacheKind" && key != "boshPackageCacheReadOnly" && key != "boshPackageCacheLocation" {
			configMap.Set(key, value.(string))
		}
	}
	containerDir, err := util.TempDir("", "fissile-stow-tests")
	fullContainerPath := filepath.Join(containerDir, "cache")
	defer os.RemoveAll(containerDir)
	p, err := NewPackageStorage(packageCacheConfig["boshPackageCacheKind"].(string), false, configMap, compilationWorkDir, fullContainerPath, imageName)
	assert.NoError(err)

	c, err := NewDockerCompilator(dockerManager, compilationWorkDir, "", imageName, compilation.FakeBase, "3.14.15", "", false, ui, nil, p)
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathBoshCache := filepath.Join(workDir, "../test-assets/bosh-cache")
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
