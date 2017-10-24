package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/SUSE/fissile/util"

	"github.com/stretchr/testify/assert"
)

func TestFinalReleaseValidationOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release")

	_, err = NewFinalRelease(emptyFinalReleasePath)

	assert.NoError(err)
}

func TestFinalReleaseSpecificNameOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release")

	release, err := NewFinalRelease(emptyFinalReleasePath)

	assert.NoError(err)
	assert.Equal("test-final", release.Name)
	assert.Equal("1", release.Version)
}

func TestFinalReleaseMissingNameInMetaData(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release-missing-release-name")

	_, err = NewFinalRelease(emptyFinalReleasePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "name does not exist in release.MF file for release")
}

func TestFinalReleaseMissingVersionInMetaData(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release-missing-release-version")

	_, err = NewFinalRelease(emptyFinalReleasePath)

	assert.NotNil(err)
	assert.Contains(err.Error(), "version does not exist in release.MF file for release")
	//assert.Equal("test-final", release.Name)
}

func TestFinalReleasePackagesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release")

	release, err := NewFinalRelease(emptyFinalReleasePath)

	assert.NoError(err)
	assert.NotNil(release)

	assert.Len(release.Packages, 2)

	barPkg, err := release.LookupPackage("bar")
	assert.NoError(err)

	assert.Equal("bar", barPkg.Name)
	assert.Equal("foo", barPkg.Dependencies[0].Name)
}

func TestFinalReleasePackageExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release")

	release, err := NewFinalRelease(emptyFinalReleasePath)

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

func TestFinalReleaseJobsOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release")

	release, err := NewFinalRelease(emptyFinalReleasePath)

	assert.NoError(err)
	assert.NotNil(release)

	assert.Len(release.Jobs, 2)

	barJob, err := release.LookupJob("bar")
	assert.NoError(err)

	assert.Equal("bar", barJob.Name)
	if assert.NotEmpty(barJob.Packages) {
		assert.Equal("bar", barJob.Packages[0].Name)
	}
}

func TestFinalReleaseJobExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	emptyFinalReleasePath := filepath.Join(workDir, "../test-assets/test-final-release")

	release, err := NewFinalRelease(emptyFinalReleasePath)

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
