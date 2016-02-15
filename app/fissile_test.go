package app

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

func TestListPackages(t *testing.T) {
	ui := termui.New(os.Stdin, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	badReleasePathCacheDir := filepath.Join(badReleasePath, "bosh-cache")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)

	err = f.ListDevPackages([]string{badReleasePath}, []string{""}, []string{""}, badReleasePathCacheDir)
	assert.Error(err, "Expected ListPackages to not find the release")

	err = f.ListDevPackages([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	assert.Nil(err, "Expected ListPackages to find the release")
}

func TestListJobs(t *testing.T) {
	ui := termui.New(os.Stdin, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	badReleasePathCacheDir := filepath.Join(badReleasePath, "bosh-cache")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")

	f := NewFissileApplication(".", ui)

	err = f.ListDevJobs([]string{badReleasePath}, []string{""}, []string{""}, badReleasePathCacheDir)
	assert.Error(err, "Expected ListJobs to not find the release")

	err = f.ListDevJobs([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	assert.Nil(err, "Expected ListJobs to find the release")
}

func TestDiffConfigurations(t *testing.T) {
	ui := termui.New(bytes.NewBufferString(""), ioutil.Discard, nil)
	assert := assert.New(t)
	workDir, err := os.Getwd()
	assetsPath := filepath.Join(workDir, "../test-assets/config-diffs")

	prefix := "hcf"
	releasePath1 := filepath.Join(assetsPath, "releases/cf-v217")
	releasePath2 := filepath.Join(assetsPath, "releases/cf-v222")
	f := NewFissileApplication("version", ui)
	hashDiffs, err := f.GetDiffConfigurationBases([]string{releasePath1, releasePath2}, prefix)

	if !assert.Nil(err, "GetDiffConfigurationBases failed") {
		return
	}
	if assert.Equal(2, len(hashDiffs.AddedKeys), fmt.Sprintf("Expected 2 added key, got %d: %s", len(hashDiffs.AddedKeys), hashDiffs.AddedKeys)) {
		sort.Strings(hashDiffs.AddedKeys)
		assert.Equal("/hcf/descriptions/nats/key_for_v222", hashDiffs.AddedKeys[0])
		assert.Equal("/hcf/spec/cf/nats/nats/key_for_v222", hashDiffs.AddedKeys[1])
	}
	if assert.Equal(2, len(hashDiffs.DeletedKeys), fmt.Sprintf("Expected 2 dropped key, got %d: %s", len(hashDiffs.DeletedKeys), hashDiffs.DeletedKeys)) {
		sort.Strings(hashDiffs.DeletedKeys)
		assert.Equal("/hcf/descriptions/nats/key_for_v217", hashDiffs.DeletedKeys[0])
		assert.Equal("/hcf/spec/cf/nats/nats/key_for_v217", hashDiffs.DeletedKeys[1])
	}
	if assert.Equal(2, len(hashDiffs.ChangedValues)) {
		v, ok := hashDiffs.ChangedValues["/hcf/spec/cf/nats/nats/debug"]
		if assert.True(ok) {
			assert.Equal("false", v[0])
			assert.Equal("true", v[1])
		}
	}
	v, ok := hashDiffs.ChangedValues["/hcf/spec/cf/nats/kingcole"]
	assert.False(ok)
	v, ok = hashDiffs.ChangedValues["/hcf/spec/cf/nats/nats/authorization_timeout"]
	assert.True(ok, fmt.Sprintf("%s", hashDiffs.ChangedValues))
	if ok {
		assert.Equal("15", v[0])
		assert.Equal("16", v[1])
	}
}

func TestDiffConfigurationsBadArgs(t *testing.T) {
	ui := termui.New(bytes.NewBufferString(""), ioutil.Discard, nil)
	assert := assert.New(t)
	prefix := "hcf"

	workDir, err := os.Getwd()
	assert.Nil(err)
	assetsPath := filepath.Join(workDir, "../test-assets/config-diffs")

	f := NewFissileApplication("version", ui)
	releasePath1 := filepath.Join(assetsPath, "releases/cf-v217")
	err = f.DiffConfigurationBases([]string{}, prefix)
	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two release paths, got 0")
	}
	err = f.DiffConfigurationBases([]string{releasePath1}, prefix)
	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two release paths, got 1")
	}
}

func TestDiffConfigurationsNoSuchPaths(t *testing.T) {
	ui := termui.New(bytes.NewBufferString(""), ioutil.Discard, nil)
	assert := assert.New(t)
	prefix := "hcf"

	workDir, err := os.Getwd()
	assert.Nil(err)
	assetsPath := filepath.Join(workDir, "../test-assets/config-diffs")

	f := NewFissileApplication("version", ui)
	releasePath1 := filepath.Join(assetsPath, "releases/cf-v217")
	badReleasePaths := []string{filepath.Join(assetsPath, "**bogus**/releases/cf-v217"), filepath.Join(assetsPath, "**bogus**/releases/cf-v222")}

	// all bad
	err = f.DiffConfigurationBases(badReleasePaths, prefix)
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Contains(err.Error(), fmt.Sprintf("Path %s (release directory) does not exist", badReleasePaths[0]))
	}
	// good rel1, bad rel2, rest bad
	err = f.DiffConfigurationBases([]string{releasePath1, badReleasePaths[1]}, prefix)
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Contains(err.Error(), fmt.Sprintf("Path %s (release directory) does not exist", badReleasePaths[1]))
	}
}

func TestDevDiffConfigurations(t *testing.T) {
	assert := assert.New(t)
	workDir, err := os.Getwd()
	assert.Nil(err)

	prefix := "hcf"
	releasePathV215 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-215")
	releasePathV224 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-224")
	cachePath := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cache")
	//ui := termui.New(bytes.NewBufferString(""), ioutil.Discard, nil)
	//f := NewFissileApplication("dev-config-diff", ui)
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

	hashDiffs, err := getDiffsFromReleases([]*model.Release{release215, release224}, prefix)
	if !assert.Nil(err, "getDiffsFromReleases failed") {
		return
	}
	if assert.Equal(4, len(hashDiffs.AddedKeys), fmt.Sprintf("Expected 4 added key, got %d: %s", len(hashDiffs.AddedKeys), hashDiffs.AddedKeys)) {
		sort.Strings(hashDiffs.AddedKeys)
		assert.Equal("/hcf/descriptions/acceptance_tests/include_route_services", hashDiffs.AddedKeys[0])
		assert.Equal("/hcf/descriptions/app_ssh/oauth_client_id", hashDiffs.AddedKeys[1])
		assert.Equal("/hcf/spec/cf/acceptance-tests/acceptance_tests/include_route_services", hashDiffs.AddedKeys[2])
		assert.Equal("/hcf/spec/cf/cloud_controller_ng/app_ssh/oauth_client_id", hashDiffs.AddedKeys[3])
	}
	if assert.Equal(4, len(hashDiffs.DeletedKeys), fmt.Sprintf("Expected 4 dropped key, got %d: %s", len(hashDiffs.DeletedKeys), hashDiffs.DeletedKeys)) {
		sort.Strings(hashDiffs.DeletedKeys)
		assert.Equal("/hcf/descriptions/acceptance_tests/old_key", hashDiffs.DeletedKeys[0])
		assert.Equal("/hcf/descriptions/networks/apps", hashDiffs.DeletedKeys[1])
		assert.Equal("/hcf/spec/cf/acceptance-tests/acceptance_tests/old_key", hashDiffs.DeletedKeys[2])
		assert.Equal("/hcf/spec/cf/cloud_controller_ng/networks/apps", hashDiffs.DeletedKeys[3])
	}
	assert.Equal(5, len(hashDiffs.ChangedValues))
	v, ok := hashDiffs.ChangedValues["/hcf/descriptions/cc/staging_upload_user"]
	if assert.True(ok) {
		assert.Equal("S3 Access key for staging droplets on AWS installs; Blobstore user for other IaaSs", v[0])
		assert.Equal("User name used to access internal endpoints of Cloud Controller to upload files when staging", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/hcf/spec/cf/cloud_controller_ng/cc/external_protocol"]
	if assert.True(ok) {
		assert.Equal("http", v[0])
		assert.Equal("https", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/hcf/spec/cf/acceptance-tests/acceptance_tests/fake_key"]
	if assert.True(ok) {
		assert.Equal("49", v[0])
		assert.Equal("10", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/hcf/descriptions/acceptance_tests/use_diego"]
	if assert.True(ok) {
		assert.Equal("Services tests push their apps using diego if enabled", v[0])
		assert.Equal("App tests push their apps using diego if enabled. Route service tests require this flag to run.", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/hcf/spec/cf/cloud_controller_ng/metron_endpoint/port"]
	if assert.True(ok) {
		assert.Equal("3456", v[0])
		assert.Equal("3457", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/hcf/spec/cf/bogus/key"]
	assert.False(ok)
}
