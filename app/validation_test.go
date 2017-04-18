package app

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	rolesManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-validation-issues.yml")
	lightManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	darkManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")
	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{torReleasePath}, []string{""}, []string{""}, torReleasePathBoshCache)
	assert.NoError(err)

	roleManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	assert.NoError(err)

	opinions, err := model.NewOpinions(lightManifestPath, darkManifestPath)
	assert.NoError(err)

	errs := f.validateManifestAndOpinions(roleManifest, opinions)

	actual := errs.Errors()
	// checkForUndefinedBOSHProperties light
	assert.Contains(actual, `light opinion 'tor.opinion': Not found: "In any BOSH release"`)
	assert.Contains(actual, `light opinion 'tor.int_opinion': Not found: "In any BOSH release"`)
	assert.Contains(actual, `light opinion 'tor.masked_opinion': Not found: "In any BOSH release"`)
	// checkForUndefinedBOSHProperties dark
	assert.Contains(actual, `dark opinion 'tor.dark-opinion': Not found: "In any BOSH release"`)
	assert.Contains(actual, `dark opinion 'tor.masked_opinion': Not found: "In any BOSH release"`)
	// checkForUndefinedBOSHProperties manifest
	assert.Contains(actual, `role-manifest 'fox': Not found: "In any BOSH release"`)
	// checkForUntemplatedDarkOpinions
	assert.Contains(actual, `properties.tor.dark-opinion: Not found: "Dark opinion is missing template in role-manifest"`)
	assert.Contains(actual, `properties.tor.masked_opinion: Not found: "Dark opinion is missing template in role-manifest"`)
	// checkForDarkInTheLight
	assert.Contains(actual, `properties.tor.masked_opinion: Forbidden: Dark opinion found in light opinions`)
	// checkForDuplicatesBetweenManifestAndLight
	assert.Contains(actual, `properties.tor.hostname: Forbidden: Role-manifest overrides opinion, remove opinion`)
	assert.Contains(actual, `properties.tor.bogus: Forbidden: Role-manifest duplicates opinion, remove from manifest`)
	// checkForUndefinedBOSHProperties light, manifest - For the bogus property used above for checkOverridden
	assert.Contains(actual, `role-manifest 'tor.bogus': Not found: "In any BOSH release"`)
	assert.Contains(actual, `light opinion 'tor.bogus': Not found: "In any BOSH release"`)
	assert.Contains(actual, `properties.tor.hostname: Forbidden: Light opinion matches default of 'localhost'`)

	// assert.Contains(actual, `XXX`) // Trigger a fail which shows the contents of `actual`. Also template for new assertion.
	assert.Len(errs, 14)
}

func TestValidationOk(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	rolesManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-validation-ok.yml")
	lightManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/good-opinions.yml")
	darkManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/good-dark-opinions.yml")

	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{torReleasePath}, []string{""}, []string{""}, torReleasePathBoshCache)
	assert.NoError(err)

	roleManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	assert.NoError(err)

	opinions, err := model.NewOpinions(lightManifestPath, darkManifestPath)
	assert.NoError(err)

	errs := f.validateManifestAndOpinions(roleManifest, opinions)

	assert.Empty(errs)
}
