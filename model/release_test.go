package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pborman/uuid"
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
	os.MkdirAll(releaseDir, 0755)

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

	// Create an empty packages dir
	err = os.MkdirAll(filepath.Join(releaseDir, packagesDir), 0755)
	assert.Nil(err)
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "jobs directory")

	// Create an empty jobs dir
	err = os.MkdirAll(filepath.Join(releaseDir, jobsDir), 0755)
	assert.Nil(err)
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

	pkg, err := release.LookupPackage("ntp-4.2.8p2")
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

	_, err = release.LookupPackage("foo")
	assert.NotNil(err)
}

func TestLookupJobOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	job, err := release.LookupJob("ntpd")
	assert.Nil(err)

	assert.Equal("ntpd", job.Name)
}

func TestLookupJobNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	_, err = release.LookupJob("foo")
	assert.NotNil(err)
}

func TestPackageDependencies(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := NewRelease(releasePath)
	assert.Nil(err)

	pkg, err := release.LookupPackage("tor")

	assert.Nil(err)
	assert.Equal(1, len(pkg.Dependencies))
	assert.Equal("libevent", pkg.Dependencies[0].Name)
}

func TestReleaseLicenseOk(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release := Release{Path: releasePath}

	err = release.loadLicense()

	assert.Nil(err)
	assert.Equal(40, len(release.License.ActualSHA1))
	assert.NotEmpty(release.License.Files)
	assert.NotNil(release.License.Files["LICENSE"])
}

func TestReleaseLicenseNotOk(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/bad-release")
	release := Release{Path: releasePath}

	err = release.loadLicense()

	if assert.NotNil(err) {
		assert.Contains(err.Error(), "unexpected EOF")
	}
	assert.Empty(release.License.Files)
}

func TestGetDeploymentConfig(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := NewRelease(releasePath)
	assert.Nil(err)

	configs := release.GetUniqueConfigs()

	assert.NotNil(configs)
	assert.Equal(4, len(configs))
}
