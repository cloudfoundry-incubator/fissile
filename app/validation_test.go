package app

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/SUSE/termui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-validation-issues.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.LightOpinions = filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	f.Options.DarkOpinions = filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.NoError(t, err)
	require.NotNil(t, f.Manifest, "error loading role manifest")

	errs := f.Validate()
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

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-validation-ok.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.LightOpinions = filepath.Join(workDir, "../test-assets/test-opinions/good-opinions.yml")
	f.Options.DarkOpinions = filepath.Join(workDir, "../test-assets/test-opinions/good-dark-opinions.yml")
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.NoError(t, err)
	require.NotNil(t, f.Manifest, "error loading role manifest")

	errs := f.Validate()
	assert.Empty(t, errs)
}

func TestValidationHash(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/hashmat.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.NoError(t, err)
	require.NotNil(t, f.Manifest, "error loading role manifest")

	f.Options.LightOpinions = filepath.Join(workDir, "../test-assets/misc/empty.yml")
	f.Options.DarkOpinions = f.Options.LightOpinions
	errs := f.Validate()

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

func TestMandatoryDescriptions(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-missing-description.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `PELERINUL: Required value: Description is required`)
}

func TestTemplateSorting(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-unsorted-templates.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `properties.tor.hostname: Forbidden: Template key does not sort before 'properties.tor.hashed_control_password'`)
}

func TestInvalidTemplateKeys(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-invalid-template-keys.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `template key for instance group myrole: Invalid value: true: Template key must be a string`)
	assert.Contains(t, err.Error(), `global template key: Invalid value: 1: Template key must be a string`)
}

func TestBadScriptReferences(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-invalid-script-reference.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `myrole script: Invalid value: "foobar.sh"`)
}

func TestBadEnvScriptReferences(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-invalid-environ-script-reference.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `myrole environment script: Invalid value: "foobar.sh"`)
}

func TestBadPostConfigScriptReferences(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-invalid-post-config-script-reference.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `myrole post config script: Invalid value: "foobar.sh"`)
}

func TestNonStringGlobalTemplate(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-invalid-global-template-type.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `global template value: Invalid value: "properties.tor.hashed_control_password": Template value must be a string`)
	assert.Contains(t, err.Error(), `global template value: Invalid value: "properties.tor.hostname": Template value must be a string`)
}

func TestNonStringInstanceGroupTemplate(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	assert.NoError(t, err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/tor-valid-instance-group-template-type.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.NoError(t, err)
}
