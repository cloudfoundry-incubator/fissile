package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"

	"github.com/SUSE/termui"
	"github.com/stretchr/testify/assert"
)

func TestCleanCacheEmpty(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)
	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if assert.NoError(err) {
		err = f.CleanCache(workDir + "compilation")
		assert.Nil(err, "Expected CleanCache to find the release")
	}
}

func TestListPackages(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	badReleasePathCacheDir := filepath.Join(badReleasePath, "bosh-cache")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)
	err = f.LoadReleases([]string{badReleasePath}, []string{""}, []string{""}, badReleasePathCacheDir)
	assert.Error(err, "Expected ListPackages to not find the release")

	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if assert.NoError(err) {
		err = f.ListPackages(util.VerbosityDefault)
		assert.Nil(err, "Expected ListPackages to find the release")
	}
}

func TestListJobs(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	badReleasePathCacheDir := filepath.Join(badReleasePath, "bosh-cache")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{badReleasePath}, []string{""}, []string{""}, badReleasePathCacheDir)
	assert.Error(err, "Expected ListJobs to not find the release")

	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if assert.NoError(err) {
		err = f.ListJobs(util.VerbosityDefault)
		assert.Nil(err, "Expected ListJobs to find the release")
	}
}

func TestListProperties(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	badReleasePathCacheDir := filepath.Join(badReleasePath, "bosh-cache")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{badReleasePath}, []string{""}, []string{""}, badReleasePathCacheDir)
	assert.Error(err, "Expected ListProperties to not find the release")

	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if assert.NoError(err) {
		err = f.ListProperties("human")
		assert.NoError(err, "Expected ListProperties to list release properties for human consumption")

		err = f.ListProperties("json")
		assert.NoError(err, "Expected ListProperties to list release properties in JSON")

		err = f.ListProperties("yaml")
		assert.NoError(err, "Expected ListProperties to list release properties in YAML")
	}
}

var testSerializeInput struct {
	releases []*model.Release
	once     sync.Once
}

func initTestSerializeInput() {
	releases := []*model.Release{
		&model.Release{
			Name: "first release",
			Jobs: model.Jobs{
				&model.Job{
					Name:        "first job",
					Description: "a first job",
					Fingerprint: "job-one",
					Templates: []*model.JobTemplate{
						&model.JobTemplate{
							SourcePath:      "/dev/urandom",
							DestinationPath: "/dev/null",
							Content:         "hello",
						},
					},
				},
			},
			Packages: model.Packages{
				&model.Package{
					Name:        "base package",
					Fingerprint: "abc",
					Path:        "/some/path",
					SHA1:        "123",
					Version:     "one",
				},
			},
			CommitHash:         "zzz",
			DevBOSHCacheDir:    "/bosh/cache/1",
			Path:               "/some/path/1",
			UncommittedChanges: false,
			Version:            "ONE",
		},
		&model.Release{
			Name: "second release",
			Jobs: model.Jobs{
				&model.Job{
					Name:        "second job",
					Description: "a second job",
					Fingerprint: "job-two",
				},
			},
			Packages: model.Packages{
				&model.Package{
					Name:        "dependent package",
					Fingerprint: "def",
					Path:        "/another/path",
					SHA1:        "456",
					Version:     "two",
				},
			},
			CommitHash:         "qqq",
			DevBOSHCacheDir:    "/bosh/cache/2",
			Path:               "/some/path/2",
			UncommittedChanges: true,
			Version:            "TWO",
		},
	}

	// Set up package dependencies + reference cycles via release
	var lastPkg *model.Package
	for _, r := range releases {
		for _, j := range r.Jobs {
			j.Packages = r.Packages
			j.Release = r
			for _, template := range j.Templates {
				template.Job = j
			}
		}
		for _, pkg := range r.Packages {
			pkg.Release = r
			if lastPkg != nil {
				pkg.Dependencies = append(pkg.Dependencies, lastPkg)
			}
			lastPkg = pkg
		}
	}

	testSerializeInput.releases = releases
}

func TestSerializePackages(t *testing.T) {
	assert := assert.New(t)
	testSerializeInput.once.Do(initTestSerializeInput)
	f := &Fissile{releases: testSerializeInput.releases}
	result, err := f.SerializePackages()
	if !assert.NoError(err) {
		return
	}
	actual, err := json.Marshal(result)
	if !assert.NoError(err) {
		return
	}
	expected := `{
		"abc": {
			"name": "base package",
			"fingerprint": "abc",
			"path": "/some/path",
			"sha1": "123",
			"version": "one",
			"dependencies": [],
			"release": "first release"
		},
		"def": {
			"name": "dependent package",
			"fingerprint": "def",
			"path": "/another/path",
			"sha1": "456",
			"version": "two",
			"dependencies": ["abc"],
			"release": "second release"
		}
	}`
	assert.JSONEq(expected, string(actual))

	_, err = (&Fissile{}).SerializeReleases()
	assert.EqualError(err, "Releases not loaded")
}

func TestSerializeReleases(t *testing.T) {
	assert := assert.New(t)
	testSerializeInput.once.Do(initTestSerializeInput)
	f := &Fissile{releases: testSerializeInput.releases}
	result, err := f.SerializeReleases()
	if !assert.NoError(err) {
		return
	}
	actual, err := json.Marshal(result)
	if !assert.NoError(err) {
		return
	}
	expected := `{
		"first release": {
			"name": "first release",
			"packages": ["abc"],
			"commitHash": "zzz",
			"devBOSHCacheDir": "/bosh/cache/1",
			"jobs": ["job-one"],
			"license": {},
			"path": "/some/path/1",
			"uncommittedChanges": false,
			"version": "ONE"
		},
		"second release": {
			"name": "second release",
			"packages": ["def"],
			"commitHash": "qqq",
			"devBOSHCacheDir": "/bosh/cache/2",
			"jobs": ["job-two"],
			"license": {},
			"path": "/some/path/2",
			"uncommittedChanges": true,
			"version": "TWO"
		}
	}`
	assert.JSONEq(expected, string(actual))

	_, err = (&Fissile{}).SerializeReleases()
	assert.EqualError(err, "Releases not loaded")
}

func TestSerializeJobs(t *testing.T) {
	assert := assert.New(t)
	testSerializeInput.once.Do(initTestSerializeInput)
	f := &Fissile{releases: testSerializeInput.releases}
	result, err := f.SerializeJobs()
	if !assert.NoError(err) {
		return
	}
	actual, err := json.Marshal(result)
	if !assert.NoError(err) {
		return
	}
	expected := `{
		"job-one": {
			"name": "first job",
			"fingerprint": "job-one",
			"packages": ["abc"],
			"release": "first release",
			"description": "a first job",
			"path": "",
			"properties": [],
			"sha1": "",
			"templates": [{
				"sourcePath": "/dev/urandom",
				"destinationPath": "/dev/null",
				"job": "job-one",
				"content": "hello"
			}],
			"version": ""
		},
		"job-two": {
			"name": "second job",
			"fingerprint": "job-two",
			"packages": ["def"],
			"release": "second release",
			"description": "a second job",
			"path": "",
			"properties": [],
			"sha1": "",
			"templates": [],
			"version": ""
		}
	}`
	assert.JSONEq(expected, string(actual))

	_, err = (&Fissile{}).SerializeJobs()
	assert.EqualError(err, "Releases not loaded")
}

func TestDevDiffConfigurations(t *testing.T) {
	assert := assert.New(t)
	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePathV215 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-215")
	releasePathV224 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-224")
	cachePath := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cache")

	release215, err := model.NewDevRelease(releasePathV215, "", "", cachePath)
	if !assert.NoError(err) {
		return
	}
	assert.NotNil(release215)
	release224, err := model.NewDevRelease(releasePathV224, "", "", cachePath)
	assert.NoError(err)
	assert.NotNil(release224)

	assert.Len(release215.Packages, 11) // temp #
	assert.Len(release224.Packages, 9)

	hashDiffs, err := getDiffsFromReleases([]*model.Release{release215, release224})
	if !assert.Nil(err, "getDiffsFromReleases failed") {
		return
	}
	if assert.Len(hashDiffs.AddedKeys, 4, fmt.Sprintf("Expected 4 added key, got %d: %s", len(hashDiffs.AddedKeys), hashDiffs.AddedKeys)) {
		sort.Strings(hashDiffs.AddedKeys)
		assert.Equal("acceptance_tests.include_route_services", hashDiffs.AddedKeys[0])
		assert.Equal("app_ssh.oauth_client_id", hashDiffs.AddedKeys[1])
		assert.Equal("cf.acceptance-tests.acceptance_tests.include_route_services", hashDiffs.AddedKeys[2])
		assert.Equal("cf.cloud_controller_ng.app_ssh.oauth_client_id", hashDiffs.AddedKeys[3])
	}
	if assert.Len(hashDiffs.DeletedKeys, 4, fmt.Sprintf("Expected 4 dropped key, got %d: %s", len(hashDiffs.DeletedKeys), hashDiffs.DeletedKeys)) {
		sort.Strings(hashDiffs.DeletedKeys)
		assert.Equal("acceptance_tests.old_key", hashDiffs.DeletedKeys[0])
		assert.Equal("cf.acceptance-tests.acceptance_tests.old_key", hashDiffs.DeletedKeys[1])
		assert.Equal("cf.cloud_controller_ng.networks.apps", hashDiffs.DeletedKeys[2])
		assert.Equal("networks.apps", hashDiffs.DeletedKeys[3])
	}
	assert.Len(hashDiffs.ChangedValues, 5)
	v, ok := hashDiffs.ChangedValues["cc.staging_upload_user"]
	if assert.True(ok) {
		assert.Equal("S3 Access key for staging droplets on AWS installs; Blobstore user for other IaaSs", v[0])
		assert.Equal("User name used to access internal endpoints of Cloud Controller to upload files when staging", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf.cloud_controller_ng.cc.external_protocol"]
	if assert.True(ok) {
		assert.Equal("http", v[0])
		assert.Equal("https", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf.acceptance-tests.acceptance_tests.fake_key"]
	if assert.True(ok) {
		assert.Equal("49", v[0])
		assert.Equal("10", v[1])
	}
	v, ok = hashDiffs.ChangedValues["acceptance_tests.use_diego"]
	if assert.True(ok) {
		assert.Equal("Services tests push their apps using diego if enabled", v[0])
		assert.Equal("App tests push their apps using diego if enabled. Route service tests require this flag to run.", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf.cloud_controller_ng.metron_endpoint.port"]
	if assert.True(ok) {
		assert.Equal("3456", v[0])
		assert.Equal("3457", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf.bogus.key"]
	assert.False(ok)
}

func TestFissileSelectRolesToBuild(t *testing.T) {
	assert := assert.New(t)
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	workDir, err := os.Getwd()
	assert.NoError(err)

	// Set up the test params
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")
	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")

	f := NewFissileApplication(",", ui)
	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if !assert.NoError(err) {
		return
	}

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases)
	if !assert.NoError(err, "Failed to load role manifest: %s", roleManifestPath) {
		return
	}

	testSamples := []struct {
		roleNames     []string
		expectedNames []string
		err           string
	}{
		{
			roleNames:     []string{"myrole", "foorole"},
			expectedNames: []string{"foorole", "myrole"},
		},
		{
			roleNames:     []string{"myrole"},
			expectedNames: []string{"myrole"},
		},
		{
			roleNames: []string{"missing_role"},
			err:       "Some roles are unknown: [missing_role]",
		},
	}

	for _, sample := range testSamples {
		results, err := roleManifest.SelectRoles(sample.roleNames)
		if sample.err != "" {
			assert.EqualError(err, sample.err, "while testing %v", sample.roleNames)
		} else {
			assert.NoError(err, "while testing %v", sample.roleNames)
			var actualNames []string
			for _, role := range results {
				actualNames = append(actualNames, role.Name)
			}
			sort.Strings(actualNames)
			assert.Equal(sample.expectedNames, actualNames, "while testing %v", sample.roleNames)
		}
	}
}

func TestFissileGetReleasesByName(t *testing.T) {
	assert := assert.New(t)
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePaths := []string{
		filepath.Join(workDir, "../test-assets/extracted-license"),
		filepath.Join(workDir, "../test-assets/extracted-license"),
	}
	cacheDir := filepath.Join(workDir, "../test-assets/extracted-license/bosh-cache")

	f := NewFissileApplication(",", ui)
	err = f.LoadReleases(releasePaths, []string{"test-dev", "test2"}, []string{}, cacheDir)
	if !assert.NoError(err) {
		return
	}

	releases, err := f.getReleasesByName([]string{"test-dev"})
	if assert.NoError(err) {
		if assert.Len(releases, 1, "should have exactly one matching release") {
			assert.Equal("test-dev", releases[0].Name, "release has unexpected name")
		}
	}
	releases, err = f.getReleasesByName([]string{"test2", "test-dev"})
	if assert.NoError(err) {
		if assert.Len(releases, 2, "should have exactly two matching releases") {
			assert.Equal("test2", releases[0].Name, "first release has unexpected name")
			assert.Equal("test-dev", releases[1].Name, "second release has unexpected name")
		}
	}
	releases, err = f.getReleasesByName([]string{})
	if assert.NoError(err) {
		if assert.Len(releases, 2, "not specifying releases should return all releases") {
			assert.Equal("test-dev", releases[0].Name, "first release has unexpected name")
			assert.Equal("test2", releases[1].Name, "second release has unexpected name")
		}
	}
	_, err = f.getReleasesByName([]string{"test-dev", "missing"})
	if assert.Error(err, "Getting a non-existant release should result in an error") {
		assert.Contains(err.Error(), "missing", "Error message should contain missing release name")
		assert.NotContains(err.Error(), "test-dev", "Error message should not contain valid release name")
	}
}
