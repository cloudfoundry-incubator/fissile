package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SUSE/fissile/testhelpers"
	"github.com/SUSE/fissile/util"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestPackageInfoOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Packages, 1)

	assert.Equal("ntp-4.2.8p2", release.Packages[0].Name)
	const ntpdFingerprint = "543219fbdaf6ec6f8af2956016055f2fb100d782"
	const ntpdVersion = ntpdFingerprint
	const ntpdSHA1 = "e41461c222b05f961350547da086569cc4264e54"
	assert.Equal(ntpdFingerprint, release.Packages[0].Version)
	assert.Equal(ntpdVersion, release.Packages[0].Fingerprint)
	assert.Equal(ntpdSHA1, release.Packages[0].SHA1)

	packagePath := filepath.Join(ntpReleasePathBoshCache, ntpdSHA1)
	assert.Equal(packagePath, release.Packages[0].Path)

	err = util.ValidatePath(packagePath, false, "")
	assert.NoError(err)
}

func TestPackageSHA1Ok(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Packages, 1)

	assert.Nil(release.Packages[0].ValidateSHA1())
}

func TestPackageSHA1NotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Packages, 1)

	// Mess up the manifest signature
	release.Packages[0].SHA1 += "foo"

	assert.NotNil(release.Packages[0].ValidateSHA1())
}

func TestPackageExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Packages, 1)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.NoError(err)
	defer os.RemoveAll(tempDir)

	packageDir, err := release.Packages[0].Extract(tempDir)
	assert.NoError(err)

	assert.Nil(util.ValidatePath(packageDir, true, ""))
	assert.Nil(util.ValidatePath(filepath.Join(packageDir, "packaging"), false, ""))
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
