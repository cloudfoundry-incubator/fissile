package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"code.google.com/p/go-uuid/uuid"
	"github.com/stretchr/testify/assert"
)

func TestReleaseValidationOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	_, err = NewRelease(ntpReleasePath)

	assert.Nil(err)
}

func TestReleaseValidationNonExistingPath(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)

	releaseDir := filepath.Join(tempDir, uuid.New())

	_, err = NewRelease(releaseDir)

	assert.NotNil(err)
	assert.Contains(err.Error(), "does not exist")
}

func TestReleaseValidationReleasePathIsAFile(t *testing.T) {
	assert := assert.New(t)

	tempFile, err := ioutil.TempFile("", "fissile-tests")
	tempFile.Close()

	assert.Nil(err)

	_, err = NewRelease(tempFile.Name())

	assert.NotNil(err)
	assert.Contains(err.Error(), "It should be a directory")
}

func TestReleaseValidationStructure(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)
	releaseDir := filepath.Join(tempDir, uuid.New())

	// Create an empty release dir
	os.MkdirAll(releaseDir, 0x755)

	release := Release{
		Path: releaseDir,
	}

	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "release manifest")

	// Create an empty manifest file
	file, err := os.Create(filepath.Join(releaseDir, manifestFile))
	assert.Nil(err)
	file.Close()
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "packages dir")

	// Create an empty packages dir
	err = os.MkdirAll(filepath.Join(releaseDir, packagesDir), 0x755)
	assert.Nil(err)
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "jobs directory")

	// Create an empty jobs dir
	err = os.MkdirAll(filepath.Join(releaseDir, jobsDir), 0x755)
	assert.Nil(err)
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "license archive")

	// Create an empty release archive
	file, err = os.Create(filepath.Join(releaseDir, licenseArchive))
	assert.Nil(err)
	file.Close()
	err = release.validatePathStructure()
	assert.Nil(err)
}

func TestReleaseMetadataOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal("ntp", release.Name)
	assert.Equal("bbece039", release.CommitHash)
	assert.Equal(false, release.UncommittedChanges)
	assert.Equal("2", release.Version)
}

func TestReleasePackagesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))
}

func TestReleaseJobsOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))
}

func TestLookupPackageOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	pkg, err := release.lookupPackage("ntp-4.2.8p2")
	assert.Nil(err)

	assert.Equal("ntp-4.2.8p2", pkg.Name)
}

func TestLookupPackageNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	_, err = release.lookupPackage("foo")
	assert.NotNil(err)
}
