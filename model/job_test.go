package model

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"code.cloudfoundry.org/fissile/testhelpers"
	"code.cloudfoundry.org/fissile/util"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

type FakeRelease struct {
	ReleasePath string
	FakeRelease Release
}

type JobInfo struct {
	Fingerprint string
	Version     string
	SHA1        string
	Path        string
}

func TestDevAndFinalReleaseJob(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpDevReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpDevReleasePathCacheDir := filepath.Join(workDir, "../test-assets/bosh-cache")
	devRelease, err := NewDevRelease(ntpDevReleasePath, "", "", ntpDevReleasePathCacheDir)
	assert.NoError(err)

	devJobPath := filepath.Join(ntpDevReleasePathCacheDir, "8eeef09ae67e65aeb3257df402336de04822efc5")
	devJobInfo := JobInfo{
		Fingerprint: "9c168f583bc177f91e6ef6ef1eab1b4550b78b1e",
		Version:     "9c168f583bc177f91e6ef6ef1eab1b4550b78b1e",
		SHA1:        "8eeef09ae67e65aeb3257df402336de04822efc5",
		Path:        devJobPath,
	}

	ntpFinalReleasePath := filepath.Join(workDir, "../test-assets/ntp-final-release")
	finalRelease, err := NewFinalRelease(ntpFinalReleasePath)
	assert.NoError(err)

	finalJobPath := filepath.Join(ntpFinalReleasePath, "jobs", finalRelease.Jobs[0].Name+".tgz")
	finalJobInfo := JobInfo{
		Fingerprint: "99939d396a864a0df92aa3724e84d0e430c9298a",
		Version:     "99939d396a864a0df92aa3724e84d0e430c9298a",
		SHA1:        "26f8c7353305424f01d03b504cfa5a5262abc83d",
		Path:        finalJobPath,
	}

	t.Run("Dev release testJobInfoOk", testJobInfoOk(devRelease, devJobInfo))
	t.Run("Dev release testJobExtractOk", testJobExtractOk(devRelease))
	t.Run("Dev release testJobSha1Ok", testJobSha1Ok(devRelease))
	t.Run("Dev release testJobSha1NotOk", testJobSha1NotOk(devRelease))
	t.Run("Dev release testJobPackagesOk", testJobPackagesOk(devRelease, "ntp-4.2.8p2"))
	t.Run("Dev release testJobTemplatesOk", testJobTemplatesOk(devRelease))
	t.Run("Dev release testJobPropertiesOk", testJobPropertiesOk(devRelease))
	t.Run("Dev release testGetJobPropertyOk", testGetJobPropertyOk(devRelease, 3))
	t.Run("Dev release testGetJobPropertyNotOk", testGetJobPropertyNotOk(devRelease, 3))
	t.Run("Dev release testJobLinksOk", testJobLinksOk(devRelease))
	t.Run("Dev release testJobsProperties", testJobsProperties(devRelease))

	t.Run("Final release testJobInfoOk", testJobInfoOk(finalRelease, finalJobInfo))
	t.Run("Final release testJobExtractOk", testJobExtractOk(finalRelease))
	t.Run("Final release testJobSha1Ok", testJobSha1Ok(finalRelease))
	t.Run("Final release testJobSha1NotOk", testJobSha1NotOk(finalRelease))
	t.Run("Final release testJobPackagesOk", testJobPackagesOk(finalRelease, "ntp"))
	t.Run("Final release testJobTemplatesOk", testJobTemplatesOk(finalRelease))
	t.Run("Final release testGetJobPropertyOk", testGetJobPropertyOk(finalRelease, 1))
	t.Run("Final release testFinalJobPropertiesOk", testFinalJobPropertiesOk(finalRelease))
	t.Run("Final release testGetJobPropertyNotOk", testGetJobPropertyNotOk(finalRelease, 1))
	t.Run("Final release testFinalJobsProperties", testFinalJobsProperties(finalRelease))
}

func testJobInfoOk(fakeRelease *Release, jobInfo JobInfo) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Equal("ntpd", fakeRelease.Jobs[0].Name)
		assert.Equal(jobInfo.Fingerprint, fakeRelease.Jobs[0].Version)
		assert.Equal(jobInfo.Version, fakeRelease.Jobs[0].Fingerprint)
		assert.Equal(jobInfo.SHA1, fakeRelease.Jobs[0].SHA1)
		assert.Equal(jobInfo.Path, fakeRelease.Jobs[0].Path)

		err := util.ValidatePath(jobInfo.Path, false, "")
		assert.NoError(err)
	}
}

func testJobSha1Ok(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Nil(fakeRelease.Jobs[0].ValidateSHA1())
	}
}

func testJobSha1NotOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)

		// Mess up the manifest signature
		fakeRelease.Jobs[0].SHA1 += "foo"
		assert.NotNil(fakeRelease.Jobs[0].ValidateSHA1())

	}
}

func testJobExtractOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)

		tempDir, err := ioutil.TempDir("", "fissile-tests")
		assert.NoError(err)
		defer os.RemoveAll(tempDir)

		jobDir, err := fakeRelease.Jobs[0].Extract(tempDir)
		assert.NoError(err)

		assert.Nil(util.ValidatePath(jobDir, true, ""))
		assert.Nil(util.ValidatePath(filepath.Join(jobDir, "job.MF"), false, ""))
	}
}

func testJobPackagesOk(fakeRelease *Release, packageName string) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Len(fakeRelease.Jobs[0].Packages, 1)
		assert.Equal(packageName, fakeRelease.Jobs[0].Packages[0].Name)
	}
}

func testJobTemplatesOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Len(fakeRelease.Jobs[0].Templates, 2)

		assert.Contains([]string{"ctl.sh", "ntp.conf.erb"}, fakeRelease.Jobs[0].Templates[0].SourcePath)
		assert.Contains([]string{"ctl.sh", "ntp.conf.erb"}, fakeRelease.Jobs[0].Templates[1].SourcePath)

		assert.Contains([]string{"etc/ntp.conf", "bin/ctl"}, fakeRelease.Jobs[0].Templates[0].DestinationPath)
		assert.Contains([]string{"etc/ntp.conf", "bin/ctl"}, fakeRelease.Jobs[0].Templates[1].DestinationPath)
	}
}

func testJobPropertiesOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Len(fakeRelease.Jobs[0].Properties, 3)

		assert.Equal("ntp_conf", fakeRelease.Jobs[0].Properties[0].Name)
		assert.Equal("ntpd's configuration file (ntp.conf)", fakeRelease.Jobs[0].Properties[0].Description)
		// Looks like properties are sorted by name.
		assert.Equal("tor.private_key", fakeRelease.Jobs[0].Properties[1].Name)
		assert.Equal("M3Efvw4x3kzW+YBWR1oPG7hoUcPcFYXWxoYkYR5+KT4=", fakeRelease.Jobs[0].Properties[1].Default)
		assert.Equal("", fakeRelease.Jobs[0].Properties[1].Description)
	}
}

func testFinalJobPropertiesOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Len(fakeRelease.Jobs[0].Properties, 1)
		assert.Equal("ntp_conf", fakeRelease.Jobs[0].Properties[0].Name)
		assert.Equal("ntpd's configuration file (ntp.conf)", fakeRelease.Jobs[0].Properties[0].Description)
	}
}

func testGetJobPropertyOk(fakeRelease *Release, jobPropertyLen int) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Len(fakeRelease.Jobs[0].Properties, jobPropertyLen)
		property, err := fakeRelease.Jobs[0].getProperty("ntp_conf")

		assert.NoError(err)
		assert.Equal("ntp_conf", property.Name)
	}
}

func testGetJobPropertyNotOk(fakeRelease *Release, jobPropertyLen int) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)
		assert.Len(fakeRelease.Jobs[0].Properties, jobPropertyLen)
		_, err := fakeRelease.Jobs[0].getProperty("foo")
		assert.NotNil(err)
		assert.Contains(err.Error(), "not found in job")
	}
}

func testJobLinksOk(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		if !assert.Len(fakeRelease.Jobs, 1) {
			return
		}

		job, err := fakeRelease.LookupJob("ntpd")
		if assert.NoError(err, "Failed to find ntpd job") {
			assert.Equal([]JobConsumesInfo{
				JobConsumesInfo{JobLinkInfo: JobLinkInfo{Name: "ntp-server", Type: "ntpd"}},
				JobConsumesInfo{JobLinkInfo: JobLinkInfo{Type: "ntp"}, Optional: true},
				JobConsumesInfo{JobLinkInfo: JobLinkInfo{Type: "missing"}, Optional: true},
			}, job.DesiredConsumers)
			assert.Equal(map[string]JobProvidesInfo{
				"ntp-server": {JobLinkInfo: JobLinkInfo{Name: "ntp-server", Type: "ntpd", JobName: "ntpd"}},
				"ntp-client": {JobLinkInfo: JobLinkInfo{Name: "ntp-client", Type: "ntp", JobName: "ntpd"}},
			}, job.AvailableProviders)
		}
	}
}

func testJobsProperties(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)

		workDir, err := os.Getwd()
		assert.NoError(err)

		lightOpinionsPath := filepath.Join(workDir, "../test-assets/ntp-opinions/opinions.yml")
		darkOpinionsPath := filepath.Join(workDir, "../test-assets/ntp-opinions/dark-opinions.yml")
		opinions, err := NewOpinions(lightOpinionsPath, darkOpinionsPath)
		assert.NoError(err)

		properties, err := fakeRelease.Jobs[0].GetPropertiesForJob(opinions)
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
}

func testFinalJobsProperties(fakeRelease *Release) func(*testing.T) {
	return func(t *testing.T) {
		assert := assert.New(t)

		assert.Len(fakeRelease.Jobs, 1)

		workDir, err := os.Getwd()
		assert.NoError(err)

		lightOpinionsPath := filepath.Join(workDir, "../test-assets/ntp-opinions/opinions.yml")
		darkOpinionsPath := filepath.Join(workDir, "../test-assets/ntp-opinions/dark-opinions.yml")
		opinions, err := NewOpinions(lightOpinionsPath, darkOpinionsPath)
		assert.NoError(err)

		properties, err := fakeRelease.Jobs[0].GetPropertiesForJob(opinions)
		assert.Len(properties, 1)
		actualJSON, err := json.Marshal(properties)
		if assert.NoError(err) {
			assert.JSONEq(`{
			"ntp_conf" : "zip.conf"
		}`, string(actualJSON), "Unexpected properties")
		}
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

func TestJobMarshal(t *testing.T) {
	testCases := []struct {
		value    *Job
		expected string
	}{
		{
			value: &Job{
				Name: "simple",
			},
			expected: `---
				name: simple
			`,
		},
		{
			value: &Job{
				Name: "release-name",
				Release: &Release{
					Name: "some-release",
				},
			},
			expected: `---
				name: release-name
				release: some-release
			`,
		},
		{
			value: &Job{
				Name: "templates",
				Templates: []*JobTemplate{
					&JobTemplate{
						SourcePath: "/source",
						Content:    "<content>",
						Job:        &Job{Name: "templates"}, // fake a loop
					},
				},
			},
			expected: `---
				name: templates
				templates:
				- sourcePath: /source
				  content: <content>
			`,
		},
		{
			value: &Job{
				Name: "packages",
				Packages: []*Package{
					{
						Fingerprint: "abc",
					},
				},
			},
			expected: `---
				name: packages
				packages:
				- abc # only list the fingerprint, not the whole object
			`,
		},
		{
			value: &Job{
				Name:        "filled-out",
				Description: "a filled-out job",
				Path:        "/path/to/thing",
				Fingerprint: "abc123",
				SHA1:        "def456",
				Properties: []*JobProperty{
					&JobProperty{
						Name:        "property",
						Description: "some job property",
						Default:     1,
						Job:         &Job{Name: "filled-out"}, // fake a loop
					},
				},
				Version: "v123",
			},
			expected: `---
				name: filled-out
				description: a filled-out job
				path: /path/to/thing
				fingerprint: abc123
				sha1: def456
				properties:
				- name: property
				  description: some job property
				  default: 1
				  job: filled-out
				version: v123
			`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.value.Name, func(t *testing.T) {
			assert := assert.New(t)
			adapter := util.NewMarshalAdapter(testCase.value)

			actual, err := yaml.Marshal(adapter)
			if !assert.NoError(err) {
				return
			}

			var unmarshalled, expected interface{}

			if !assert.NoError(yaml.Unmarshal(actual, &unmarshalled), "Error unmarshalling result") {
				return
			}
			expectedBytes := []byte(strings.Replace(testCase.expected, "\t", "    ", -1))
			if !assert.NoError(yaml.Unmarshal(expectedBytes, &expected), "Error in expected input") {
				return
			}
			testhelpers.IsYAMLSubset(assert, expected, unmarshalled)
		})
	}
}
