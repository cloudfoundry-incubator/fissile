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

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

	assert.Equal("ntp-4.2.8p2", release.Packages[0].Name)
	assert.Equal("543219fbdaf6ec6f8af2956016055f2fb100d782", release.Packages[0].Version)
	assert.Equal("543219fbdaf6ec6f8af2956016055f2fb100d782", release.Packages[0].Fingerprint)
	assert.Equal("e42db26038a42994b0255939d0d046ca58071a45", release.Packages[0].SHA1)

	packagePath := filepath.Join(ntpReleasePath, packagesDir, "ntp-4.2.8p2.tgz")
	assert.Equal(packagePath, release.Packages[0].Path)

	err = util.ValidatePath(packagePath, false, "")
	assert.Nil(err)
}

func TestPackageSHA1Ok(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

	assert.Nil(release.Packages[0].ValidateSHA1())
}

func TestPackageSHA1NotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

	// Mess up the manifest signature
	release.Packages[0].SHA1 += "foo"

	assert.NotNil(release.Packages[0].ValidateSHA1())
}

func TestPackageLoadLicenseFiles(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal("libevent", release.Packages[0].Name)
	assert.Equal(1, len(release.Packages[0].LicenseFiles))
	assert.Equal([]byte("license file\n"), release.Packages[0].LicenseFiles["./libevent/LICENSE"])
}

func TestPackageExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Packages))

	tempDir, err := ioutil.TempDir("", "fissile-tests")

	packageDir, err := release.Packages[0].Extract(tempDir)
	assert.Nil(err)

	assert.Nil(util.ValidatePath(packageDir, true, ""))
	assert.Nil(util.ValidatePath(filepath.Join(packageDir, "packaging"), false, ""))
}
