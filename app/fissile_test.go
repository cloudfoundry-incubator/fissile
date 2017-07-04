package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/SUSE/fissile/model"
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
		err = f.ListPackages(false)
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
		err = f.ListJobs(false)
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
