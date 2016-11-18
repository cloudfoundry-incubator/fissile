package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/util"

	"github.com/stretchr/testify/assert"
)

func TestPackageInfoOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

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
	assert.Nil(err)
}

func TestPackageSHA1Ok(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

	assert.Nil(release.Packages[0].ValidateSHA1())
}

func TestPackageSHA1NotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

	// Mess up the manifest signature
	release.Packages[0].SHA1 += "foo"

	assert.NotNil(release.Packages[0].ValidateSHA1())
}

func TestPackageExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)
	defer os.RemoveAll(tempDir)

	packageDir, err := release.Packages[0].Extract(tempDir)
	assert.Nil(err)

	assert.Nil(util.ValidatePath(packageDir, true, ""))
	assert.Nil(util.ValidatePath(filepath.Join(packageDir, "packaging"), false, ""))
}
