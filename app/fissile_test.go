package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

func TestCleanCacheEmpty(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

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
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	badReleasePathCacheDir := filepath.Join(badReleasePath, "bosh-cache")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)
	err = f.LoadReleases([]string{badReleasePath}, []string{""}, []string{""}, badReleasePathCacheDir)
	assert.Error(err, "Expected ListPackages to not find the release")

	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if assert.NoError(err) {
		err = f.ListPackages()
		assert.Nil(err, "Expected ListPackages to find the release")
	}
}

func TestListJobs(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	badReleasePathCacheDir := filepath.Join(badReleasePath, "bosh-cache")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{badReleasePath}, []string{""}, []string{""}, badReleasePathCacheDir)
	assert.Error(err, "Expected ListJobs to not find the release")

	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if assert.NoError(err) {
		err = f.ListJobs()
		assert.Nil(err, "Expected ListJobs to find the release")
	}
}

func TestListProperties(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

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
	assert.Nil(err)

	releasePathV215 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-215")
	releasePathV224 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-224")
	cachePath := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cache")

	release215, err := model.NewDevRelease(releasePathV215, "", "", cachePath)
	if !assert.Nil(err) {
		return
	}
	assert.NotNil(release215)
	release224, err := model.NewDevRelease(releasePathV224, "", "", cachePath)
	assert.Nil(err)
	assert.NotNil(release224)

	assert.Equal(11, len(release215.Packages)) // temp #
	assert.Equal(9, len(release224.Packages))

	hashDiffs, err := getDiffsFromReleases([]*model.Release{release215, release224})
	if !assert.Nil(err, "getDiffsFromReleases failed") {
		return
	}
	if assert.Equal(4, len(hashDiffs.AddedKeys), fmt.Sprintf("Expected 4 added key, got %d: %s", len(hashDiffs.AddedKeys), hashDiffs.AddedKeys)) {
		sort.Strings(hashDiffs.AddedKeys)
		assert.Equal("/descriptions/acceptance_tests/include_route_services", hashDiffs.AddedKeys[0])
		assert.Equal("/descriptions/app_ssh/oauth_client_id", hashDiffs.AddedKeys[1])
		assert.Equal("/spec/cf/acceptance-tests/acceptance_tests/include_route_services", hashDiffs.AddedKeys[2])
		assert.Equal("/spec/cf/cloud_controller_ng/app_ssh/oauth_client_id", hashDiffs.AddedKeys[3])
	}
	if assert.Equal(4, len(hashDiffs.DeletedKeys), fmt.Sprintf("Expected 4 dropped key, got %d: %s", len(hashDiffs.DeletedKeys), hashDiffs.DeletedKeys)) {
		sort.Strings(hashDiffs.DeletedKeys)
		assert.Equal("/descriptions/acceptance_tests/old_key", hashDiffs.DeletedKeys[0])
		assert.Equal("/descriptions/networks/apps", hashDiffs.DeletedKeys[1])
		assert.Equal("/spec/cf/acceptance-tests/acceptance_tests/old_key", hashDiffs.DeletedKeys[2])
		assert.Equal("/spec/cf/cloud_controller_ng/networks/apps", hashDiffs.DeletedKeys[3])
	}
	assert.Equal(5, len(hashDiffs.ChangedValues))
	v, ok := hashDiffs.ChangedValues["/descriptions/cc/staging_upload_user"]
	if assert.True(ok) {
		assert.Equal("S3 Access key for staging droplets on AWS installs; Blobstore user for other IaaSs", v[0])
		assert.Equal("User name used to access internal endpoints of Cloud Controller to upload files when staging", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/spec/cf/cloud_controller_ng/cc/external_protocol"]
	if assert.True(ok) {
		assert.Equal("http", v[0])
		assert.Equal("https", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/spec/cf/acceptance-tests/acceptance_tests/fake_key"]
	if assert.True(ok) {
		assert.Equal("49", v[0])
		assert.Equal("10", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/descriptions/acceptance_tests/use_diego"]
	if assert.True(ok) {
		assert.Equal("Services tests push their apps using diego if enabled", v[0])
		assert.Equal("App tests push their apps using diego if enabled. Route service tests require this flag to run.", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/spec/cf/cloud_controller_ng/metron_endpoint/port"]
	if assert.True(ok) {
		assert.Equal("3456", v[0])
		assert.Equal("3457", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/spec/cf/bogus/key"]
	assert.False(ok)
}
