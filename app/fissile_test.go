package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
	lightOpinionsPaths := [2]string{filepath.Join(assetsPath, "opinions/cf-v217/opinions.yml"), filepath.Join(assetsPath, "opinions/cf-v222/opinions.yml")}
	darkOpinionsPaths := [2]string{filepath.Join(assetsPath, "opinions/cf-v217/dark-opinions.yml"), filepath.Join(assetsPath, "opinions/cf-v222/dark-opinions.yml")}
	configStore := configstore.NewConfigStoreBuilder(prefix, "", lightOpinionsPaths[0], darkOpinionsPaths[0], "")
	hashDiffs, err := configStore.DiffConfigurations(releasePath1, releasePath2, lightOpinionsPaths[1], darkOpinionsPaths[1])

	if !assert.Nil(err, "DiffConfigurations failed") {
		return
	}
	assert.Equal(1, len(hashDiffs.AddedKeys))
	assert.Equal("/hcf/opinions/nats/trace", hashDiffs.AddedKeys[0])
	assert.Equal(1, len(hashDiffs.DeletedKeys))
	assert.Equal("/hcf/opinions/nats/monitor_port", hashDiffs.DeletedKeys[0])
	assert.Equal(2, len(hashDiffs.ChangedValues))
	v, ok := hashDiffs.ChangedValues["/hcf/opinions/nats/port"]
	assert.True(ok)
	if ok {
		assert.Equal("4222", v[0])
		assert.Equal("beefalo", v[1])
	}
	v, ok = hashDiffs.ChangedValues["/hcf/opinions/nats/kingcole"]
	assert.False(ok)
	v, ok = hashDiffs.ChangedValues["/hcf/opinions/nats/machines"]
	assert.True(ok)
	if ok {
		assert.Equal("[\"0.0.0.2\"]", v[0])
		assert.Equal("[\"0.0.0.3\"]", v[1])
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
	releasePath2 := filepath.Join(assetsPath, "releases/cf-v222")
	releasePaths := []string{releasePath1, releasePath2}
	lightOpinionsPaths := []string{filepath.Join(assetsPath, "opinions/cf-v217/opinions.yml"), filepath.Join(assetsPath, "opinions/cf-v222/opinions.yml")}
	darkOpinionsPaths := []string{filepath.Join(assetsPath, "opinions/cf-v217/dark-opinions.yml"), filepath.Join(assetsPath, "opinions/cf-v222/dark-opinions.yml")}
	err = f.DiffConfigurationBases([]string{}, lightOpinionsPaths, darkOpinionsPaths, prefix)
	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two release paths, got 0")
	}
	err = f.DiffConfigurationBases([]string{releasePath1}, lightOpinionsPaths, darkOpinionsPaths, prefix)
	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two release paths, got 1")
	}
	err = f.DiffConfigurationBases(releasePaths, []string{}, darkOpinionsPaths, prefix)
	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two light-opinion paths, got 0")
	}
	err = f.DiffConfigurationBases(releasePaths, []string{lightOpinionsPaths[0]}, darkOpinionsPaths, prefix)
	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two light-opinion paths, got 1")
	}
	err = f.DiffConfigurationBases(releasePaths, []string{}, []string{}, prefix)
	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two light-opinion paths, got 0")
		assert.Contains(err.Error(), "expected two dark-opinion paths, got 0")
	}
	err = f.DiffConfigurationBases([]string{}, []string{}, []string{}, prefix)

	if assert.Error(err, "Expected an error for bad args") {
		assert.Contains(err.Error(), "expected two release paths, got 0")
		assert.Contains(err.Error(), "expected two light-opinion paths, got 0")
		assert.Contains(err.Error(), "expected two dark-opinion paths, got 0")
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
	releasePath2 := filepath.Join(assetsPath, "releases/cf-v222")
	releasePaths := []string{releasePath1, releasePath2}
	lightOpinionsPaths := []string{filepath.Join(assetsPath, "opinions/cf-v217/opinions.yml"), filepath.Join(assetsPath, "opinions/cf-v222/opinions.yml")}
	darkOpinionsPaths := []string{filepath.Join(assetsPath, "opinions/cf-v217/dark-opinions.yml"), filepath.Join(assetsPath, "opinions/cf-v222/dark-opinions.yml")}
	badReleasePaths := []string{filepath.Join(assetsPath, "**bogus**/releases/cf-v217"), filepath.Join(assetsPath, "**bogus**/releases/cf-v222")}
	badLightPaths := []string{filepath.Join(assetsPath, "**bogus**/opinions/cf-v217/opinions.yml"), filepath.Join(assetsPath, "**bogus**/opinions/cf-v222/opinions.yml")}
	badDarkPaths := []string{filepath.Join(assetsPath, "**bogus**/opinions/cf-v217/dark-opinions.yml"), filepath.Join(assetsPath, "**bogus**/opinions/cf-v222/dark-opinions.yml")}

	// all bad
	err = f.DiffConfigurationBases(badReleasePaths, badLightPaths, badDarkPaths, prefix)
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Contains(err.Error(), fmt.Sprintf("Path %s (release directory) does not exist", badReleasePaths[0]))
	}
	// good rel1, bad rel2, rest bad
	err = f.DiffConfigurationBases([]string{releasePath1, badReleasePaths[1]}, badLightPaths, badDarkPaths, prefix)
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Contains(err.Error(), fmt.Sprintf("Path %s (release directory) does not exist", badReleasePaths[1]))
	}
	// good rel, rest bad
	err = f.DiffConfigurationBases(releasePaths, badLightPaths, badDarkPaths, prefix)
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Equal(err.Error(), fmt.Sprintf("open %s: no such file or directory", badLightPaths[0]))
	}
	// good rel, good first dark, rest bad
	err = f.DiffConfigurationBases(releasePaths, badLightPaths, []string{darkOpinionsPaths[0], badDarkPaths[1]}, "prefix")
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Equal(err.Error(), fmt.Sprintf("open %s: no such file or directory", badLightPaths[0]))
	}
	// good rel, first light good, first dark good
	err = f.DiffConfigurationBases(releasePaths, []string{lightOpinionsPaths[0], badLightPaths[1]}, []string{darkOpinionsPaths[0], badDarkPaths[1]}, "prefix")
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Equal(err.Error(), fmt.Sprintf("open %s: no such file or directory", badLightPaths[1]))
	}
	// good rel, good light, first dark good
	err = f.DiffConfigurationBases(releasePaths, lightOpinionsPaths, []string{darkOpinionsPaths[0], badDarkPaths[1]}, "prefix")
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Equal(err.Error(), fmt.Sprintf("open %s: no such file or directory", badDarkPaths[1]))
	}
	// good rel, good dark, first light good
	err = f.DiffConfigurationBases(releasePaths, []string{lightOpinionsPaths[0], badLightPaths[1]}, darkOpinionsPaths, "prefix")
	if assert.Error(err, "Expected an error for bogus paths") {
		assert.Equal(err.Error(), fmt.Sprintf("open %s: no such file or directory", badLightPaths[1]))
	}
}
