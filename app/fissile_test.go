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

// *****************************************************************************
// Test changes that require rebuilding images

type differentHashType [3]string

var beforeHashCache map[string]string

func getBeforeHash(ui *termui.UI) (map[string]string, error) {
	if beforeHashCache != nil {
		return beforeHashCache, nil
	}
	tmp, err := getFullHash(ui, "before")
	if err != nil {
		return nil, err
	}
	beforeHashCache = tmp
	return beforeHashCache, err
}

func getFullHash(ui *termui.UI, targetDir string) (map[string]string, error) {
	workDir, err := os.Getwd()
	baseDir := filepath.Join(workDir, "../test-assets/detect-changes-configs/")
	releasePath := filepath.Join(baseDir, "ntp-release")
	releasePathCacheDir := filepath.Join(releasePath, "bosh-cache")
	f := NewFissileApplication(".", ui)
	err = f.LoadReleases([]string{releasePath}, []string{""}, []string{""}, releasePathCacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load releases from %s:%s", releasePath, err)
	}
	if len(f.releases) != 1 {
		return nil, fmt.Errorf("expected to load 1 release, got %d\n", len(f.releases))
	}
	configBaseDir := filepath.Join(baseDir, "small01")
	mainConfigDir := filepath.Join(configBaseDir, targetDir)
	rolesManifestPath := filepath.Join(mainConfigDir, "role-manifest.yml")
	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	if err != nil {
		return nil, fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}
	lightManifestPath := filepath.Join(mainConfigDir, "opinions.yml")
	darkManifestPath := filepath.Join(mainConfigDir, "dark-opinions.yml")
	err = rolesManifest.SetGlobalConfig(lightManifestPath, darkManifestPath)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, role := range rolesManifest.Roles {
		versionHash, err := role.GetRoleDevVersion()
		if err != nil {
			return nil, err
		}
		result[role.Name] = versionHash
	}
	return result, nil
}

func doTestImageChangesOneRoleChange(t *testing.T, targetDir string) {
	assert := assert.New(t)
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	beforeHash, err := getBeforeHash(ui)
	if !assert.Nil(err) {
		return
	}
	afterHash, err := getFullHash(ui, targetDir)
	if !assert.Nil(err) {
		return
	}
	droppedKeys, addedKeys, commonKeys, differentValues := getCacheDiffs(beforeHash, afterHash) // Keys are related to roles
	assert.Equal(0, len(droppedKeys))
	assert.Equal(0, len(addedKeys))
	if assert.Equal(1, len(commonKeys)) {
		assert.Equal("ntpd", commonKeys[0])
	}
	if assert.Equal(1, len(differentValues)) {
		assert.Equal("ntpd", differentValues[0][0])
	}
}

func doTestImageChangesZeroRoleChanges(t *testing.T, targetDir string) {
	assert := assert.New(t)
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	beforeHash, err := getBeforeHash(ui)
	if !assert.Nil(err) {
		return
	}
	afterHash, err := getFullHash(ui, targetDir)
	if !assert.Nil(err) {
		return
	}
	droppedKeys, addedKeys, commonKeys, differentValues := getCacheDiffs(beforeHash, afterHash) // Keys are related to roles
	assert.Equal(0, len(droppedKeys))
	assert.Equal(0, len(addedKeys))
	if assert.Equal(1, len(commonKeys)) {
		assert.Equal("ntpd", commonKeys[0])
	}
	assert.Equal(0, len(differentValues))
}

func TestImageChangesAddPropertyToManifest(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-new-property-in-manifest")
}

func TestImageChangesAddPropertyToOpinions(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-new-property-in-opinions")
}

func TestImageChangesDropPropertyFromConfigs(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-property-dropped")
}

func TestImageChangesOpinionValueChanged(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-prop-value-change")
}

func TestNoImageChangesAfterRunChange(t *testing.T) {
	doTestImageChangesZeroRoleChanges(t, "after-run-change")
}

func TestImageChangesAfterScriptListChange(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-scriptlist-change")
}

func TestImageChangesAfterScriptContentsChange(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-script-change")
}

func TestImageAddDarkOpinionValueChanged(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-dark-opinion-add")
}

func TestImageDropDarkOpinionValueChanged(t *testing.T) {
	doTestImageChangesOneRoleChange(t, "after-dark-opinion-drop")
}

func TestNoImageChangesAfterDarkOpinionValueChanged(t *testing.T) {
	doTestImageChangesZeroRoleChanges(t, "after-dark-opinion-value-change")
}

func getCacheDiffs(oldKeys, newKeys map[string]string) ([]string, []string, []string, []differentHashType) {
	droppedKeys := []string{}
	addedKeys := []string{}
	commonKeys := []string{}
	differentValues := []differentHashType{}
	for k, oldV := range oldKeys {
		newV, ok := newKeys[k]
		if ok {
			commonKeys = append(commonKeys, k)
			if oldV != newV {
				differentValues = append(differentValues, differentHashType{k, oldV, newV})
			}
		} else {
			droppedKeys = append(droppedKeys, k)
		}
	}
	for k := range newKeys {
		_, ok := oldKeys[k]
		if !ok {
			addedKeys = append(addedKeys, k)
		}
	}
	return droppedKeys, addedKeys, commonKeys, differentValues
}
