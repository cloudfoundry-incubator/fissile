package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/util"

	"github.com/stretchr/testify/assert"
)

func TestDevReleaseValidationOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "test-dev", "", emptyDevReleaseCachePath)

	assert.Nil(err)
}

func TestDevReleaseLatestVersionOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "test-dev", "", emptyDevReleaseCachePath)

	assert.Nil(err)
	assert.NotNil(release)
	assert.Equal("0+dev.2", release.Version)
}

func TestDevReleaseSpecificVersionOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "test-dev", "0+dev.1", emptyDevReleaseCachePath)

	assert.Nil(err)
	assert.NotNil(release)
	assert.Equal("0+dev.1", release.Version)
}

func TestDevReleaseValidationNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "foo-dev", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "release dev manifests directory")
}

func TestDevReleasePackagesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "test-dev", "", emptyDevReleaseCachePath)

	assert.Nil(err)
	assert.NotNil(release)

	assert.Equal(2, len(release.Packages))

	barPkg, err := release.LookupPackage("bar")
	assert.Nil(err)

	assert.Equal("bar", barPkg.Name)
	assert.Equal("foo", barPkg.Dependencies[0].Name)
}

func TestDevReleasePackageExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "test-dev", "", emptyDevReleaseCachePath)

	assert.Nil(err)
	assert.NotNil(release)

	assert.Equal(2, len(release.Packages))

	barPkg, err := release.LookupPackage("bar")
	assert.Nil(err)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)

	extractedPath, err := barPkg.Extract(tempDir)
	assert.Nil(err)

	assert.Nil(util.ValidatePath(extractedPath, true, "extracted package dir"))
}

func TestDevReleaseJobsOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "test-dev", "", emptyDevReleaseCachePath)

	assert.Nil(err)
	assert.NotNil(release)

	assert.Equal(2, len(release.Jobs))

	barJob, err := release.LookupJob("bar")
	assert.Nil(err)

	assert.Equal("bar", barJob.Name)
	assert.Equal("bar", barJob.Packages[0].Name)
}

func TestDevReleaseJobExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "test-dev", "", emptyDevReleaseCachePath)

	assert.Nil(err)
	assert.NotNil(release)

	assert.Equal(2, len(release.Packages))

	barJob, err := release.LookupJob("bar")
	assert.Nil(err)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)

	extractedPath, err := barJob.Extract(tempDir)
	assert.Nil(err)

	assert.Nil(util.ValidatePath(extractedPath, true, "extracted job dir"))
}
