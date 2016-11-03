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

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	_, err = NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)

	assert.Nil(err)
}

func TestReleaseValidationNonExistingPath(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)
	defer os.RemoveAll(tempDir)

	releaseDir := filepath.Join(tempDir, uuid.New())
	releaseDirBoshCache := filepath.Join(releaseDir, "bosh-cache")

	_, err = NewDevRelease(releaseDir, "", "", releaseDirBoshCache)

	assert.NotNil(err)
	assert.Contains(err.Error(), "does not exist")
}

func TestReleaseValidationReleasePathIsAFile(t *testing.T) {
	assert := assert.New(t)

	tempFile, err := ioutil.TempFile("", "fissile-tests")
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	assert.Nil(err)

	_, err = NewDevRelease(tempFile.Name(), "", "", "")

	assert.NotNil(err)
	assert.Contains(err.Error(), "It should be a directory")
}

func TestReleaseValidationStructure(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)
	defer os.RemoveAll(tempDir)
	releaseDir := filepath.Join(tempDir, uuid.New())

	// Create an empty release dir
	os.MkdirAll(releaseDir, 0755)

	release := Release{
		Path:    releaseDir,
		Name:    "test",
		Version: "0",
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

	// Create an empty manifest file
	os.MkdirAll(filepath.Join(releaseDir, "dev_releases", "test"), 0755)
	err = ioutil.WriteFile(
		filepath.Join(releaseDir, "dev_releases", "test", "test-0.yml"),
		[]byte{},
		0644,
	)
	assert.Nil(err)
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "(packages directory) does not exist")

	// Create an empty packages dir
	err = os.MkdirAll(filepath.Join(releaseDir, packagesDir), 0755)
	assert.Nil(err)
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "(jobs directory) does not exist")

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

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	// These values from test-assets/ntp-release/dev_releases/ntp/ntp-2+dev.3.yml
	assert.Equal("ntp", release.Name)
	assert.Equal("036e7564", release.CommitHash)
	assert.Equal(true, release.UncommittedChanges)
	assert.Equal("2+dev.3", release.Version)
}

func TestReleasePackagesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))
}

func TestReleaseJobsOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))
}

func TestLookupPackageOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	pkg, err := release.LookupPackage("ntp-4.2.8p2")
	assert.Nil(err)

	assert.Equal("ntp-4.2.8p2", pkg.Name)
}

func TestLookupPackageNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	_, err = release.LookupPackage("foo")
	assert.NotNil(err)
}

func TestLookupJobOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	job, err := release.LookupJob("ntpd")
	assert.Nil(err)

	assert.Equal("ntpd", job.Name)
}

func TestLookupJobNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	_, err = release.LookupJob("foo")
	assert.NotNil(err)
}

func TestPackageDependencies(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)
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

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	release := Release{Path: releasePath}

	err = release.loadLicense()

	assert.Nil(err)
	assert.NotEmpty(release.License.Files)
	assert.NotNil(release.License.Files["LICENSE"])
}

func TestReleaseNoLicense(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/no-license")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)

	assert.Nil(err, "Release without license should be valid")
	assert.Empty(release.License.Files)
}

func TestReleaseExtractedLicense(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/extracted-license")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)

	assert.Nil(err, "Release with extracted license should be valid")
	assert.Equal(1, len(release.License.Files))
	assert.Equal([]byte("LICENSE file contents"), release.License.Files["LICENSE"])
}

func TestReleaseMissingLicense(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/missing-license")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	_, err = NewDevRelease(releasePath, "", "", releasePathBoshCache)

	assert.NotNil(err, "Release with missing license should be invalid")
}

func TestGetDeploymentConfig(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.Nil(err)

	configs := release.GetUniqueConfigs()

	assert.NotNil(configs)
	assert.Equal(4, len(configs))
}
