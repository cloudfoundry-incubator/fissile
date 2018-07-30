package app

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/termui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/app/tor-validation-issues.yml")
	lightManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	darkManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")
	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{torReleasePath}, []string{""}, []string{""}, torReleasePathBoshCache)
	assert.NoError(t, err)

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases, f)
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	opinions, err := model.NewOpinions(lightManifestPath, darkManifestPath)
	assert.NoError(t, err)

	errs := f.validateManifestAndOpinions(roleManifest, opinions)

	actual := errs.Errors()
	allExpected := []string{
		// checkForUndefinedBOSHProperties light
		`light opinion 'tor.opinion': Not found: "In any BOSH release"`,
		`light opinion 'tor.int_opinion': Not found: "In any BOSH release"`,
		`light opinion 'tor.masked_opinion': Not found: "In any BOSH release"`,
		// checkForUndefinedBOSHProperties dark
		`dark opinion 'tor.dark-opinion': Not found: "In any BOSH release"`,
		`dark opinion 'tor.masked_opinion': Not found: "In any BOSH release"`,
		// checkForUndefinedBOSHProperties manifest
		`role-manifest 'fox': Not found: "In any BOSH release"`,
		// checkForUntemplatedDarkOpinions
		`properties.tor.dark-opinion: Not found: "Dark opinion is missing template in role-manifest"`,
		`properties.tor.masked_opinion: Not found: "Dark opinion is missing template in role-manifest"`,
		// checkForDarkInTheLight
		`properties.tor.masked_opinion: Forbidden: Dark opinion found in light opinions`,
		// checkForDuplicatesBetweenManifestAndLight
		`configuration.templates[properties.tor.hostname]: Forbidden: Role-manifest overrides opinion, remove opinion`,
		`instance-groups[myrole].configuration.templates[properties.tor.bogus]: Forbidden: Role-manifest duplicates opinion, remove from manifest`,
		// checkForUndefinedBOSHProperties light, manifest - For the bogus property used above for checkOverridden
		`role-manifest 'tor.bogus': Not found: "In any BOSH release"`,
		`light opinion 'tor.bogus': Not found: "In any BOSH release"`,
		`properties.tor.hostname: Forbidden: Light opinion matches default of 'localhost'`,

		// `XXX`, // Trigger a fail which shows the contents of `actual`. Also template for new assertions.
	}
	for _, expected := range allExpected {
		assert.Contains(t, actual, expected)
	}
	assert.Len(t, errs, len(allExpected))
}

func TestValidationOk(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/app/tor-validation-ok.yml")
	lightManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/good-opinions.yml")
	darkManifestPath := filepath.Join(workDir, "../test-assets/test-opinions/good-dark-opinions.yml")

	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{torReleasePath}, []string{""}, []string{""}, torReleasePathBoshCache)
	assert.NoError(t, err)

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases, f)
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	opinions, err := model.NewOpinions(lightManifestPath, darkManifestPath)
	assert.NoError(t, err)

	errs := f.validateManifestAndOpinions(roleManifest, opinions)

	assert.Empty(t, errs)
}

func TestValidationHash(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/app/hashmat.yml")
	emptyManifestPath := filepath.Join(workDir, "../test-assets/misc/empty.yml")
	f := NewFissileApplication(".", ui)

	err = f.LoadReleases([]string{torReleasePath}, []string{""}, []string{""}, torReleasePathBoshCache)
	assert.NoError(t, err)

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases, f)
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	opinions, err := model.NewOpinions(emptyManifestPath, emptyManifestPath)
	assert.NoError(t, err)

	errs := f.validateManifestAndOpinions(roleManifest, opinions)

	actual := errs.Errors()
	allExpected := []string{
		`role-manifest 'not.a.hash.foo': Not found: "In any BOSH release"`,
		// `XXX`, // Trigger a fail which shows the contents of `actual`. Also template for new assertions.
	}
	for _, expected := range allExpected {
		assert.Contains(t, actual, expected)
	}
	assert.Len(t, errs, len(allExpected))
}
