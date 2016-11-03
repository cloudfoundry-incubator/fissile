package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hpcloud/fissile/util"

	"github.com/stretchr/testify/assert"
)

func TestJobInfoOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	// Hashes taken from test-assets/ntp-release/dev_releases/ntp/ntp-2+dev.3.yml
	ntpdJobFingerprint := "9c168f583bc177f91e6ef6ef1eab1b4550b78b1e"
	ntpdJobSha1Hash := "c4c278b2d10c7aea06d41b6f655037fcd642aa0f"

	assert.Equal("ntpd", release.Jobs[0].Name)
	assert.Equal(ntpdJobFingerprint, release.Jobs[0].Version)
	assert.Equal(ntpdJobFingerprint, release.Jobs[0].Fingerprint)
	assert.Equal(ntpdJobSha1Hash, release.Jobs[0].SHA1)

	jobPath := filepath.Join(ntpReleasePathCacheDir, ntpdJobSha1Hash)
	assert.Equal(jobPath, release.Jobs[0].Path)

	err = util.ValidatePath(jobPath, false, "")
	assert.Nil(err)
}

func TestJobSha1Ok(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Nil(release.Jobs[0].ValidateSHA1())
}

func TestJobSha1NotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
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

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
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

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(1, len(release.Jobs[0].Packages))
	assert.Equal("ntp-4.2.8p2", release.Jobs[0].Packages[0].Name)
}

func TestJobTemplatesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
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

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(3, len(release.Jobs[0].Properties))

	assert.Equal("ntp_conf", release.Jobs[0].Properties[0].Name)
	assert.Equal("ntpd's configuration file (ntp.conf)", release.Jobs[0].Properties[0].Description)
}

func TestGetJobPropertyOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	// properties from test-assets/ntp-release/jobs/ntpd/spec
	names := []string{"ntp_conf", "with.json.default", "tor.private_key"}
	job := release.Jobs[0]
	for _, name := range names {
		property, err := job.getProperty(name)
		if assert.Nil(err) {
			assert.Equal(name, property.Name)
		}
	}
}

func TestGetJobPropertyNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))
	_, err = release.Jobs[0].getProperty("foo")

	if assert.NotNil(err) {
		assert.Contains(err.Error(), "not found in job")
	}
}

func TestJobsSort(t *testing.T) {
	assert := assert.New(t)

	jobs := Jobs{
		{Name: "aaa"},
		{Name: "bbb"},
	}
	sort.Sort(jobs)
	assert.Equal(jobs[0].Name, "aaa")
	assert.Equal(jobs[1].Name, "bbb")

	jobs = Jobs{
		{Name: "ddd"},
		{Name: "ccc"},
	}
	sort.Sort(jobs)
	assert.Equal(jobs[0].Name, "ccc")
	assert.Equal(jobs[1].Name, "ddd")
}
