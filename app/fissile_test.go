package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hpcloud/fissile/config-store"
	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

func TestListPackages(t *testing.T) {
	ui := termui.New(os.Stdin, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")

	f := NewFissileApplication(".", ui)

	err = f.ListPackages(badReleasePath)
	assert.Error(err, "Expected ListPackages to not find the release")

	err = f.ListPackages(releasePath)
	assert.Nil(err, "Expected ListPackages to find the release")
}

func TestListJobs(t *testing.T) {
	ui := termui.New(os.Stdin, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	releasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")

	f := NewFissileApplication(".", ui)

	err = f.ListJobs(badReleasePath)
	assert.Error(err, "Expected ListJobs to not find the release")

	err = f.ListJobs(releasePath)
	assert.Nil(err, "Expected ListJobs to find the release")
}

func TestListFullConfiguration(t *testing.T) {
	ui := termui.New(os.Stdin, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")

	f := NewFissileApplication(".", ui)

	err = f.ListFullConfiguration(badReleasePath)
	assert.Error(err, "Expected ListFullConfiguration to not find the release")

	err = f.ListFullConfiguration(releasePath)
	assert.Nil(err, "Expected ListFullConfiguration to find the release")
}

func TestPrintTemplateReport(t *testing.T) {
	ui := termui.New(os.Stdin, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	badReleasePath := filepath.Join(workDir, "../test-assets/bad-release")
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")

	f := NewFissileApplication(".", ui)

	err = f.PrintTemplateReport(badReleasePath)
	assert.Error(err, "Expected PrintTemplateReport to not find the release")

	err = f.PrintTemplateReport(releasePath)
	assert.Nil(err, "Expected PrintTemplateReport to find the release")
}

func TestVerifyRelease(t *testing.T) {
	ui := termui.New(bytes.NewBufferString(""), ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)
	assetsPath := filepath.Join(workDir, "../test-assets/corrupt-releases")

	f := NewFissileApplication(".", ui)

	err = f.VerifyRelease(filepath.Join(assetsPath, "valid-release"))
	assert.Nil(err, "Expected valid release to be verifiable")

	err = f.VerifyRelease(filepath.Join(assetsPath, "corrupt-job"))
	assert.Error(err, "Expected corrupt job to fail release verification")
	assert.Contains(fmt.Sprintf("%v", err), "corrupt_job.tgz")

	err = f.VerifyRelease(filepath.Join(assetsPath, "corrupt-package"))
	assert.Error(err, "Expected corrupt package to fail release verification")
	assert.Contains(fmt.Sprintf("%v", err), "corrupt_package.tgz")

	err = f.VerifyRelease(filepath.Join(assetsPath, "corrupt-license"))
	assert.Error(err, "Expected corrupt license to fail release verification")
	assert.Contains(fmt.Sprintf("%v", err), "license")
}

func TestDiffConfigurations(t *testing.T) {
	assert := assert.New(t)
	workDir, err := os.Getwd()
	assetsPath := filepath.Join(workDir, "../test-assets/config-diffs")

	prefix := "hcf"
	releasePath1 := filepath.Join(assetsPath, "releases/cf-v217")
	releasePath2 := filepath.Join(assetsPath, "releases/cf-v222")
	configStore := configstore.NewConfigStoreBuilder(prefix, "", "", "", "")
	hashDiffs, err := configStore.DiffConfigurations(releasePath1, releasePath2)

	if !assert.Nil(err, "DiffConfigurations failed") {
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
