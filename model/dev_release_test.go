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
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NoError(err)
}

func TestDevReleaseLatestVersionOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.NotNil(release)
	assert.Equal("test-dev", release.Name)
	assert.Equal("0+dev.2", release.Version)
}

func TestDevReleaseSpecificVersionOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "", "0+dev.1", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.NotNil(release)
	assert.Equal("0+dev.1", release.Version)
}

func TestDevReleaseSpecificNameOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "test2", "", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.Equal("test2", release.Name)
	assert.Equal("0+dev.1", release.Version)
}

func TestDevReleaseValidationNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "foo-dev", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "release dev manifests directory")
}

func TestDevReleaseValidationBadConfigNoDevNameKeyWithFinalName(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release-missing-dev-name")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.Equal("test-final", release.Name)
}

func TestDevReleaseValidationBadConfigNoDevNameKeyNoFinalName(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release-missing-final-name")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "neither name nor final_name key exists in the configuration file for release")
}

func TestDevReleaseValidationBadConfigWrongFinalNameType(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release-wrong-final-name-type")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "final_name was not a string in release")
}

func TestDevReleaseValidationBadIndexNoBuildsKey(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "bad-index-no-builds-key", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "builds key did not exist in dev releases index file for release")
}

func TestDevReleaseValidationBadIndexWrongBuildsKeyType(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "bad-index-wrong-builds-key-type", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "builds key in dev releases index file was not a map for release")
}

func TestDevReleaseValidationBadIndexNoVersionKeyInBuild(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "bad-index-no-version-in-build", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "version key did not exist in a build entry for release")
}

func TestDevReleaseValidationBadIndexWrongVersionTypeInBuild(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	_, err = NewDevRelease(emptyDevReleasePath, "bad-index-wrong-version-type-in-build", "", emptyDevReleaseCachePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "version was not a string in a build entry for release")
}

func TestDevReleasePackagesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.NotNil(release)

	assert.Len(release.Packages, 2)

	barPkg, err := release.LookupPackage("bar")
	assert.NoError(err)

	assert.Equal("bar", barPkg.Name)
	assert.Equal("foo", barPkg.Dependencies[0].Name)
}

func TestDevReleasePackageExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.NotNil(release)

	assert.Len(release.Packages, 2)

	barPkg, err := release.LookupPackage("bar")
	assert.NoError(err)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.NoError(err)
	defer os.RemoveAll(tempDir)

	extractedPath, err := barPkg.Extract(tempDir)
	assert.NoError(err)

	assert.Nil(util.ValidatePath(extractedPath, true, "extracted package dir"))
}

func TestDevReleaseJobsOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.NotNil(release)

	assert.Len(release.Jobs, 2)

	barJob, err := release.LookupJob("bar")
	assert.NoError(err)

	assert.Equal("bar", barJob.Name)
	assert.Equal("bar", barJob.Packages[0].Name)
}

func TestDevReleaseJobExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyDevReleasePath := filepath.Join(workDir, "../test-assets/test-dev-release")
	emptyDevReleaseCachePath := filepath.Join(workDir, "../test-assets/test-dev-release-cache")

	release, err := NewDevRelease(emptyDevReleasePath, "", "", emptyDevReleaseCachePath)

	assert.NoError(err)
	assert.NotNil(release)

	assert.Len(release.Packages, 2)

	barJob, err := release.LookupJob("bar")
	assert.NoError(err)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.NoError(err)
	defer os.RemoveAll(tempDir)

	extractedPath, err := barJob.Extract(tempDir)
	assert.NoError(err)

	assert.Nil(util.ValidatePath(extractedPath, true, "extracted job dir"))
}
