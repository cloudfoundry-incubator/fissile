package compilator

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/SUSE/fissile/docker"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/scripts/compilation"
	"github.com/SUSE/fissile/util"

	"github.com/graymeta/stow"
	"github.com/graymeta/stow/local"
	"github.com/graymeta/stow/s3"
	"github.com/stretchr/testify/assert"
)

func TestStorePackageLocallyOK(t *testing.T) {
	// Arrange
	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.NoError(t, err)
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.NoError(t, err)
	imageName := "splatform/fissile-stemcell-opensuse:42.2/"

	packageCacheConfigFilename := filepath.Join(os.TempDir(), "s3example.json")
	packageCacheConfigReader, err := ioutil.ReadFile(packageCacheConfigFilename)
	if err != nil {
		panic(err)
	}

	var packageCacheConfig map[string]interface{}

	if err := json.Unmarshal(packageCacheConfigReader, &packageCacheConfig); err != nil {
		panic(err)
	}
	var config stow.Config
	if packageCacheConfig["kind"].(string) == "local" {
		config = stow.ConfigMap{local.ConfigKeyPath: packageCacheConfig["configKeyPath"].(string)}
	} else {
		if packageCacheConfig["kind"].(string) == "s3" {
			config = stow.ConfigMap{
				s3.ConfigAccessKeyID: packageCacheConfig["access_key_id"].(string),
				s3.ConfigSecretKey:   packageCacheConfig["secret_key"].(string),
				s3.ConfigRegion:      packageCacheConfig["region"].(string),
			}
		}
	}
	containerLocation := packageCacheConfig["boshCompiledPackageLocation"].(string)

	p, err := NewPackageStorage(packageCacheConfig["kind"].(string), config, compilationWorkDir, containerLocation, imageName)
	if err != nil {
		panic(err)
	}

	c, err := NewDockerCompilator(dockerManager, compilationWorkDir, "", imageName, compilation.FakeBase, "3.14.15", "", false, ui, nil, p)
	assert.NoError(t, err)
	workDir, err := os.Getwd()
	assert.NoError(t, err)
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(t, err)

	// Act
	err = c.compilePackageInDocker(release.Packages[0])
	// Upload (stow)
	pack := release.Packages[0]
	defer p.location.Close()

	err = p.Upload(pack)
	if err != nil {
		panic(err)
	}

	err = p.Download(pack)
	if err != nil {
		panic(err)
	}

	exists, err := p.Exists(pack)
	if err != nil {
		panic(err)
	}
	//Assert
	assert.True(t, exists)
	assert.NoError(t, err)
}

func TestStorePackageExists(t *testing.T) {
	//Arrange
	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.NoError(t, err)
	imageName := "splatform/fissile-stemcell-opensuse:42.2/"
	//
	packageCacheConfigFilename := filepath.Join(os.TempDir(), "localexample.json")
	packageCacheConfigReader, err := ioutil.ReadFile(packageCacheConfigFilename)
	if err != nil {
		panic(err)
	}

	var packageCacheConfig map[string]interface{}

	if err := json.Unmarshal(packageCacheConfigReader, &packageCacheConfig); err != nil {
		panic(err)
	}
	var config stow.Config
	if packageCacheConfig["kind"].(string) == "local" {
		config = stow.ConfigMap{local.ConfigKeyPath: packageCacheConfig["configKeyPath"].(string)}
	} else {
		if packageCacheConfig["kind"].(string) == "s3" {
			config = stow.ConfigMap{
				s3.ConfigAccessKeyID: packageCacheConfig["access_key_id"].(string),
				s3.ConfigSecretKey:   packageCacheConfig["secret_key"].(string),
				s3.ConfigRegion:      packageCacheConfig["region"].(string),
			}
		}
	}
	containerLocation := packageCacheConfig["boshCompiledPackageLocation"].(string)

	p, err := NewPackageStorage(packageCacheConfig["kind"].(string), config, compilationWorkDir, containerLocation, imageName)
	if err != nil {
		panic(err)
	}
	//
	c, err := NewDockerCompilator(dockerManager, compilationWorkDir, "", imageName, compilation.FakeBase, "3.14.15", "", false, ui, nil, p)
	assert.NoError(t, err)
	workDir, err := os.Getwd()
	assert.NoError(t, err)
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(t, err)
	//Act
	err = c.compilePackageInDocker(release.Packages[0])
	pack := release.Packages[0]
	defer p.location.Close()
	existsFalse, err := p.Exists(pack)
	if err != nil {
		panic(err)
	}

	p.Upload(pack)

	existsTrue, err := p.Exists(pack)
	if err != nil {
		panic(err)
	}

	//Assert
	assert.False(t, existsFalse)
	assert.True(t, existsTrue)
}
