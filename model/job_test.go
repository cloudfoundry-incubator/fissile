package model

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/SUSE/fissile/util"

	"strings"

	"github.com/stretchr/testify/assert"
)

func TestJobInfoOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)
	const ntpdFingerprint = "9c168f583bc177f91e6ef6ef1eab1b4550b78b1e"
	const ntpdVersion = ntpdFingerprint
	const ntpdSHA1 = "aab8da0094ac318f790ca40c53f7a5f4e137f841"

	assert.Equal("ntpd", release.Jobs[0].Name)
	assert.Equal(ntpdFingerprint, release.Jobs[0].Version)
	assert.Equal(ntpdVersion, release.Jobs[0].Fingerprint)
	assert.Equal(ntpdSHA1, release.Jobs[0].SHA1)

	jobPath := filepath.Join(ntpReleasePathCacheDir, ntpdSHA1)
	assert.Equal(jobPath, release.Jobs[0].Path)

	err = util.ValidatePath(jobPath, false, "")
	assert.NoError(err)
}

func TestJobSha1Ok(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	assert.Nil(release.Jobs[0].ValidateSHA1())
}

func TestJobSha1NotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	// Mess up the manifest signature
	release.Jobs[0].SHA1 += "foo"

	assert.NotNil(release.Jobs[0].ValidateSHA1())
}

func TestJobExtractOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	tempDir, err := ioutil.TempDir("", "fissile-tests")
	assert.NoError(err)
	defer os.RemoveAll(tempDir)

	jobDir, err := release.Jobs[0].Extract(tempDir)
	assert.NoError(err)

	assert.Nil(util.ValidatePath(jobDir, true, ""))
	assert.Nil(util.ValidatePath(filepath.Join(jobDir, "job.MF"), false, ""))
}

func TestJobPackagesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	assert.Len(release.Jobs[0].Packages, 1)
	assert.Equal("ntp-4.2.8p2", release.Jobs[0].Packages[0].Name)
}

func TestJobTemplatesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathCacheDir := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathCacheDir)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	assert.Len(release.Jobs[0].Templates, 2)

	assert.Contains([]string{"ctl.sh", "ntp.conf.erb"}, release.Jobs[0].Templates[0].SourcePath)
	assert.Contains([]string{"ctl.sh", "ntp.conf.erb"}, release.Jobs[0].Templates[1].SourcePath)

	assert.Contains([]string{"etc/ntp.conf", "bin/ctl"}, release.Jobs[0].Templates[0].DestinationPath)
	assert.Contains([]string{"etc/ntp.conf", "bin/ctl"}, release.Jobs[0].Templates[1].DestinationPath)
}

func TestJobPropertiesOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	assert.Len(release.Jobs[0].Properties, 3)

	assert.Equal("ntp_conf", release.Jobs[0].Properties[0].Name)
	assert.Equal("ntpd's configuration file (ntp.conf)", release.Jobs[0].Properties[0].Description)
	// Looks like properties are sorted by name.
	assert.Equal("tor.private_key", release.Jobs[0].Properties[1].Name)
	assert.Equal("M3Efvw4x3kzW+YBWR1oPG7hoUcPcFYXWxoYkYR5+KT4=", release.Jobs[0].Properties[1].Default)
	assert.Equal("", release.Jobs[0].Properties[1].Description)
}

func TestGetJobPropertyOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	assert.Len(release.Jobs[0].Properties, 3)

	property, err := release.Jobs[0].getProperty("ntp_conf")

	assert.NoError(err)
	assert.Equal("ntp_conf", property.Name)
}

func TestGetJobPropertyNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	assert.Len(release.Jobs[0].Properties, 3)

	_, err = release.Jobs[0].getProperty("foo")

	assert.NotNil(err)
	assert.Contains(err.Error(), "not found in job")
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

func TestJobsProperties(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	lightOpinionsPath := filepath.Join(workDir, "../test-assets/ntp-opinions/opinions.yml")
	darkOpinionsPath := filepath.Join(workDir, "../test-assets/ntp-opinions/dark-opinions.yml")
	opinions, err := NewOpinions(lightOpinionsPath, darkOpinionsPath)
	assert.NoError(err)

	properties, err := release.Jobs[0].getPropertiesForJob(opinions)
	assert.Len(properties, 2)
	actualJSON, err := json.Marshal(properties)
	if assert.NoError(err) {
		assert.JSONEq(`{
			"ntp_conf" : "zip.conf",
			"with": {
				"json": {
					"default": { "key": "value" }
				}
			}
		}`, string(actualJSON), "Unexpected properties")
	}
}

func TestWriteConfigs(t *testing.T) {
	assert := assert.New(t)

	job := &Job{
		Name: "silly job",
		Properties: []*JobProperty{
			&JobProperty{
				Name:    "prop",
				Default: "bar",
			},
		},
	}

	role := &Role{
		Name: "dummy role",
		Jobs: Jobs{job},
	}

	tempFile, err := ioutil.TempFile("", "fissile-job-test")
	assert.NoError(err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(strings.Replace(`---
	properties:
		foo: 3
	`, "\t", "    ", -1))
	assert.NoError(err)
	assert.NoError(tempFile.Close())

	json, err := job.WriteConfigs(role, tempFile.Name(), tempFile.Name())
	assert.NoError(err)

	assert.JSONEq(`
	{
		"job": {
			"name": "dummy role",
			"templates": [{
				"name": "silly job"
			}]
		},
		"parameters": {},
		"properties": {
			"prop": "bar"
		},
		"networks": {
			"default": {}
		}
	}`, string(json))
}
