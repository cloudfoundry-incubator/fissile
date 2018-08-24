package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/SUSE/fissile/testhelpers"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

type ReleaseInfo struct {
	Name               string
	CommitHash         string
	UncommittedChanges bool
	Version            string
}

func TestRelease(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpDevReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpDevReleasePathCacheDir := filepath.Join(ntpDevReleasePath, "bosh-cache")
	devRelease, err := NewDevRelease(ntpDevReleasePath, "", "", ntpDevReleasePathCacheDir)
	assert.NoError(err)

	devReleaseInfo := ReleaseInfo{
		Name:               "ntp",
		CommitHash:         "45e25267",
		UncommittedChanges: true,
		Version:            "2+dev.3",
	}

	ntpFinalReleasePath := filepath.Join(workDir, "../test-assets/ntp-final-release")
	finalRelease, err := NewFinalRelease(ntpFinalReleasePath)
	assert.NoError(err)

	finalReleaseInfo := ReleaseInfo{
		Name:               "ntp",
		CommitHash:         "84166e52",
		UncommittedChanges: true,
		Version:            "4",
	}

	t.Run("Dev release testReleaseMetadataOk", testReleaseMetadataOk(devRelease, devReleaseInfo))
	t.Run("Dev release testReleasePackagesOk", testReleasePackagesOk(devRelease))
	t.Run("Dev release testReleaseJobsOk", testReleaseJobsOk(devRelease))
	t.Run("Dev release testLookupPackageOk", testLookupPackageOk(devRelease, "ntp-4.2.8p2"))
	t.Run("Dev release testLookupPackageNotOk", testLookupPackageNotOk(devRelease))
	t.Run("Dev release testLookupJobOk", testLookupJobOk(devRelease))
	t.Run("Dev release testLookupJobNotOk", testLookupJobNotOk(devRelease))

	t.Run("Final release testReleaseMetadataOk", testReleaseMetadataOk(finalRelease, finalReleaseInfo))
	t.Run("Final release testReleasePackagesOk", testReleasePackagesOk(finalRelease))
	t.Run("Final release testReleaseJobsOk", testReleaseJobsOk(finalRelease))
	t.Run("Final release testLookupPackageOk", testLookupPackageOk(finalRelease, "ntp"))
	t.Run("Final release testLookupPackageNotOk", testLookupPackageNotOk(finalRelease))
	t.Run("Final release testLookupJobOk", testLookupJobOk(finalRelease))
	t.Run("Final release testLookupJobNotOk", testLookupJobNotOk(finalRelease))
}

func TestReleaseValidationNonExistingPath(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.NoError(err)
	defer os.RemoveAll(tempDir)

	releaseDir := filepath.Join(tempDir, uuid.New())
	releaseDirBoshCache := filepath.Join(releaseDir, "bosh-cache")

	_, err = NewDevRelease(releaseDir, "", "", releaseDirBoshCache)

	assert.NotNil(err)
	assert.Contains(err.Error(), "does not exist")

	_, err = NewFinalRelease(releaseDir)

	assert.NotNil(err)
	assert.Contains(err.Error(), "no such file or directory")
}

func TestReleaseValidationReleasePathIsAFile(t *testing.T) {
	assert := assert.New(t)

	tempFile, err := ioutil.TempFile("", "fissile-tests")
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	assert.NoError(err)

	_, err = NewDevRelease(tempFile.Name(), "", "", "")

	assert.NotNil(err)
	assert.Contains(err.Error(), "It should be a directory")

	_, err = NewFinalRelease(tempFile.Name())

	assert.NotNil(err)
	assert.Contains(err.Error(), "not a directory")
}

func TestReleaseValidationStructure(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.NoError(err)
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
	assert.NoError(err)
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
	assert.NoError(err)
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "(packages directory) does not exist")

	// Create an empty packages dir
	err = os.MkdirAll(filepath.Join(releaseDir, packagesDir), 0755)
	assert.NoError(err)
	err = release.validatePathStructure()
	assert.NotNil(err)
	assert.Contains(err.Error(), "(jobs directory) does not exist")

	// Create an empty jobs dir
	err = os.MkdirAll(filepath.Join(releaseDir, jobsDir), 0755)
	assert.NoError(err)
	err = release.validatePathStructure()
	assert.NoError(err)
}

func testReleaseMetadataOk(fakeRelease *Release, releaseInfo ReleaseInfo) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		// For dev release, these values come from the
		// RELEASE-DIR/dev_releases/RELEASE-NAME/REL-V1+dev.V2.yml
		assert.Equal(releaseInfo.Name, fakeRelease.Name)
		assert.Equal(releaseInfo.CommitHash, fakeRelease.CommitHash)
		assert.Equal(releaseInfo.UncommittedChanges, fakeRelease.UncommittedChanges)
		assert.Equal(releaseInfo.Version, fakeRelease.Version)
	}
}

func testReleasePackagesOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)
		assert.Len(fakeRelease.Packages, 1)
	}
}

func testReleaseJobsOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)
		assert.Len(fakeRelease.Jobs, 1)
	}
}

func testLookupPackageOk(fakeRelease *Release, packageName string) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		pkg, err := fakeRelease.LookupPackage(packageName)
		assert.NoError(err)

		assert.Equal(packageName, pkg.Name)
	}
}

func testLookupPackageNotOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		_, err := fakeRelease.LookupPackage("foo")
		assert.NotNil(err)
	}
}

func testLookupJobOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		job, err := fakeRelease.LookupJob("ntpd")
		assert.NoError(err)

		assert.Equal("ntpd", job.Name)
	}
}

func testLookupJobNotOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		_, err := fakeRelease.LookupJob("foo")
		assert.NotNil(err)
	}
}

func TestPackageDependencies(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	pkg, err := release.LookupPackage("tor")

	assert.NoError(err)
	assert.Len(pkg.Dependencies, 1)
	assert.Equal("libevent", pkg.Dependencies[0].Name)
}

func TestReleaseLicenseOk(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	release := Release{Path: releasePath}

	err = release.loadLicense()

	assert.NoError(err)
	assert.NotEmpty(release.License.Files)
	assert.NotNil(release.License.Files["LICENSE"])
}

func TestReleaseNoLicense(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

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
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/extracted-license")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)

	assert.Nil(err, "Release with extracted license should be valid")
	assert.Len(release.License.Files, 1)
	assert.Equal([]byte("LICENSE file contents"), release.License.Files["LICENSE"])
}

func TestReleaseMissingLicense(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/missing-license")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	_, err = NewDevRelease(releasePath, "", "", releasePathBoshCache)

	assert.NotNil(err, "Release with missing license should be invalid")
}

func TestGetDeploymentConfig(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	configs := release.GetUniqueConfigs()

	assert.NotNil(configs)
	allExpected := []string{
		`is.a.hash`,
		`its.a.hash`,
		`not.a.hash`,
		`tor.client_keys`,
		`tor.hashed_control_password`,
		`tor.hostname`,
		`tor.private_key`,
	}
	for _, expected := range allExpected {
		assert.Contains(configs, expected)
	}
	assert.Len(configs, len(allExpected))
}

func TestReleaseMarshal(t *testing.T) {
	assert := assert.New(t)
	sample := &Release{
		Jobs: Jobs{
			&Job{
				Fingerprint: "abc",
				Packages: Packages{
					&Package{
						Fingerprint: "ghi",
					},
				},
			},
			&Job{
				Fingerprint: "def",
				Packages: Packages{
					&Package{
						Fingerprint: "jkl",
					},
				},
			},
		},
		Packages:           Packages{},
		Name:               "sample release",
		UncommittedChanges: true,
		CommitHash:         "mno",
		Version:            "123",
		Path:               "/some/path",
		DevBOSHCacheDir:    "/some/bosh/cache",
	}
	// Make sure reference cycles don't break marshalling
	for _, job := range sample.Jobs {
		job.Release = sample
		for _, pkg := range job.Packages {
			pkg.Release = sample
			sample.Packages = append(sample.Packages, pkg)
		}
	}
	license := ReleaseLicense{
		Release: sample,
		Files: map[string][]byte{
			"hello": []byte("world"),
		},
	}
	sample.License = license
	expected := map[string]interface{}{
		"jobs":               []string{"abc", "def"},
		"packages":           []string{"ghi", "jkl"},
		"license":            map[string]string{"hello": "world"},
		"name":               "sample release",
		"uncommittedChanges": true,
		"commitHash":         "mno",
		"version":            "123",
		"path":               "/some/path",
		"devBOSHCacheDir":    "/some/bosh/cache",
	}
	actual, err := sample.Marshal()
	if assert.NoError(err) {
		testhelpers.IsYAMLSubset(assert, expected, actual)
	}
}
