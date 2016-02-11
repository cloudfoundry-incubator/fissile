package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/util"

	"github.com/stretchr/testify/assert"
)

func TestJobInfoOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal("ntpd", release.Jobs[0].Name)
	assert.Equal("80450dd04a4b0248b67ab4a8cc1f8b2cfb4deea5", release.Jobs[0].Version)
	assert.Equal("80450dd04a4b0248b67ab4a8cc1f8b2cfb4deea5", release.Jobs[0].Fingerprint)
	assert.Equal("b47366e160f9d139c8ce8bef4a8fef1f72e0f151", release.Jobs[0].SHA1)

	jobPath := filepath.Join(ntpReleasePath, jobsDir, "ntpd.tgz")
	assert.Equal(jobPath, release.Jobs[0].Path)

	err = util.ValidatePath(jobPath, false, "")
	assert.Nil(err)
}

func TestJobSha1Ok(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Nil(release.Jobs[0].ValidateSHA1())
}

func TestJobSha1NotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	// Mess up the manifest signature
	release.Jobs[0].SHA1 += "foo"

	assert.NotNil(release.Jobs[0].ValidateSHA1())
}

func TestJobExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)
	defer os.RemoveAll(tempDir)

	jobDir, err := release.Jobs[0].Extract(tempDir)
	assert.Nil(err)

	assert.Nil(util.ValidatePath(jobDir, true, ""))
	assert.Nil(util.ValidatePath(filepath.Join(jobDir, "job.MF"), false, ""))
}

func TestJobPackagesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(1, len(release.Jobs[0].Packages))
	assert.Equal("ntp-4.2.8p2", release.Jobs[0].Packages[0].Name)
}

func TestJobTemplatesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(2, len(release.Jobs[0].Templates))

	assert.Contains([]string{"ctl.sh", "ntp.conf.erb"}, release.Jobs[0].Templates[0].SourcePath)
	assert.Contains([]string{"ctl.sh", "ntp.conf.erb"}, release.Jobs[0].Templates[1].SourcePath)

	assert.Contains([]string{"etc/ntp.conf", "bin/ctl"}, release.Jobs[0].Templates[0].DestinationPath)
	assert.Contains([]string{"etc/ntp.conf", "bin/ctl"}, release.Jobs[0].Templates[1].DestinationPath)
}

func TestJobPropertiesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(1, len(release.Jobs[0].Properties))

	assert.Equal("ntp_conf", release.Jobs[0].Properties[0].Name)
	assert.Equal("ntpd's configuration file (ntp.conf)", release.Jobs[0].Properties[0].Description)
}

func TestGetJobPropertyOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(1, len(release.Jobs[0].Properties))

	property, err := release.Jobs[0].getProperty("ntp_conf")

	assert.Nil(err)
	assert.Equal("ntp_conf", property.Name)
}

func TestGetJobPropertyNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := NewRelease(ntpReleasePath)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(1, len(release.Jobs[0].Properties))

	_, err = release.Jobs[0].getProperty("foo")

	assert.NotNil(err)
	assert.Contains(err.Error(), "not found in job")
}
