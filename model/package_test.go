package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.cloudfoundry.org/fissile/testhelpers"
	"code.cloudfoundry.org/fissile/util"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

type PackageInfo struct {
	Name        string
	Fingerprint string
	Version     string
	SHA1        string
	Path        string
}

func TestDevAndFinalReleasePackage(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpDevReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpDevReleasePathCacheDir := filepath.Join(workDir, "../test-assets/bosh-cache")
	devRelease, err := NewDevRelease(ntpDevReleasePath, "", "", ntpDevReleasePathCacheDir)
	assert.NoError(err)

	devPackagePath := filepath.Join(ntpDevReleasePathCacheDir, "e41461c222b05f961350547da086569cc4264e54")
	devPackageInfo := PackageInfo{
		Name:        "ntp-4.2.8p2",
		Fingerprint: "543219fbdaf6ec6f8af2956016055f2fb100d782",
		Version:     "543219fbdaf6ec6f8af2956016055f2fb100d782",
		SHA1:        "e41461c222b05f961350547da086569cc4264e54",
		Path:        devPackagePath,
	}

	ntpFinalReleasePath := filepath.Join(workDir, "../test-assets/ntp-final-release")
	finalRelease, err := NewFinalRelease(ntpFinalReleasePath)
	assert.NoError(err)

	finalPackagePath := filepath.Join(ntpFinalReleasePath, "packages", "ntp.tgz")
	finalPackageInfo := PackageInfo{
		Name:        "ntp",
		Fingerprint: "7ffae62452202409eedb58be773db7d1bfd890b7",
		Version:     "7ffae62452202409eedb58be773db7d1bfd890b7",
		SHA1:        "0285ebed26d7c8d21c2a3b8f5648ad9105d49a8d",
		Path:        finalPackagePath,
	}

	t.Run("Dev release testPackageInfoOk", testPackageInfoOk(devRelease, devPackageInfo))
	t.Run("Dev release testPackageSHA1Ok", testPackageSHA1Ok(devRelease))
	t.Run("Dev release testPackageSHA1NotOk", testPackageSHA1NotOk(devRelease))
	t.Run("Dev release testPackageExtractOk", testPackageExtractOk(devRelease))

	t.Run("Final release testPackageInfoOk", testPackageInfoOk(finalRelease, finalPackageInfo))
	t.Run("Final release testPackageSHA1Ok", testPackageSHA1Ok(finalRelease))
	t.Run("Final release testPackageSHA1NotOk", testPackageSHA1NotOk(finalRelease))
	t.Run("Final release testPackageExtractOk", testPackageExtractOk(finalRelease))

}

func testPackageInfoOk(fakeRelease *Release, packageInfo PackageInfo) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Packages, 1)
		assert.Equal(packageInfo.Name, fakeRelease.Packages[0].Name)
		assert.Equal(packageInfo.Fingerprint, fakeRelease.Packages[0].Fingerprint)
		assert.Equal(packageInfo.Version, fakeRelease.Packages[0].Version)
		assert.Equal(packageInfo.SHA1, fakeRelease.Packages[0].SHA1)
		assert.Equal(packageInfo.Path, fakeRelease.Packages[0].Path)
		err := util.ValidatePath(packageInfo.Path, false, "")
		assert.NoError(err)
	}
}

func testPackageSHA1Ok(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)
		assert.Len(fakeRelease.Packages, 1)
		assert.Nil(fakeRelease.Packages[0].ValidateSHA1())
	}
}

func testPackageSHA1NotOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)
		assert.Len(fakeRelease.Packages, 1)
		// Mess up the manifest signature
		fakeRelease.Packages[0].SHA1 += "foo"
		assert.NotNil(fakeRelease.Packages[0].ValidateSHA1())
	}
}

func testPackageExtractOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)
		tempDir, err := ioutil.TempDir("", "fissile-tests")
		assert.NoError(err)
		defer os.RemoveAll(tempDir)

		packageDir, err := fakeRelease.Packages[0].Extract(tempDir)
		assert.NoError(err)

		assert.Nil(util.ValidatePath(packageDir, true, ""))
		assert.Nil(util.ValidatePath(filepath.Join(packageDir, "packaging"), false, ""))
	}
}

func TestPackageMarshal(t *testing.T) {
	assert := assert.New(t)
	sample := &Package{
		Name:        "sample package",
		Version:     "abc",
		Fingerprint: "def",
		SHA1:        "ghi",
		Release: &Release{
			Name:    "sample release",
			Version: "unused",
		},
		Path: "/some/path",
		Dependencies: Packages{
			&Package{
				Name:        "dependent package",
				Fingerprint: "jkl",
			},
		},
	}
	sample.Dependencies = append(sample.Dependencies, sample) // Create a loop
	var expected interface{}
	err := yaml.Unmarshal([]byte(strings.Replace(`---
		name: sample package
		version: abc
		fingerprint: def
		sha1: ghi
		release: sample release
		path: /some/path
		dependencies:
		- jkl
		- def
	`, "\t", "    ", -1)), &expected)
	if !assert.NoError(err, "Error unmarshalling expected value") {
		return
	}

	actual, err := sample.Marshal()
	if assert.NoError(err) {
		testhelpers.IsYAMLSubset(assert, expected, actual)
	}
}
