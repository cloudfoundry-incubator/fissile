package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/SUSE/termui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestValidation(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)

	workDir, err := os.Getwd()
	require.NoError(t, err)

	roleManifestDir, err := os.Open(filepath.Join(workDir, "../test-assets/role-manifests/app/validation/"))
	assert.NoError(t, err, "failed to open role manifest assets directory")
	require.NotNil(t, roleManifestDir)

	roleManifestNames, err := roleManifestDir.Readdirnames(0)
	assert.NoError(t, err, "failed to list role manifest assets")
	for _, roleManifestName := range roleManifestNames {
		if filepath.Ext(roleManifestName) != ".yml" {
			continue
		}
		func(testName string) {
			t.Run(testName, func(t *testing.T) {
				t.Parallel()

				roleManifestPath := filepath.Join(roleManifestDir.Name(), testName+".yml")
				roleManifestContents, err := ioutil.ReadFile(roleManifestPath)
				assert.NoError(t, err, "failed to read role manifest")

				var testData struct {
					Errors        []string               `yaml:"expected_errors"`
					LightOpinions map[string]interface{} `yaml:"light_opinions"`
					DarkOpinions  map[string]interface{} `yaml:"dark_opinions"`
				}
				err = yaml.Unmarshal(roleManifestContents, &testData)
				assert.NoError(t, err, "error reading test data")
				sort.Strings(testData.Errors)

				lightOpinions, err := ioutil.TempFile("", fmt.Sprintf("fissile-%s-*.yml", testName))
				assert.NoError(t, err, "failed to create temporary light opinions")
				require.NotNil(t, lightOpinions, "nil temporary light opinions")
				defer os.Remove(lightOpinions.Name())
				lightOpinionsEncoder := yaml.NewEncoder(lightOpinions)
				assert.NoError(t, lightOpinionsEncoder.Encode(testData.LightOpinions), "error encoding light opinions")
				assert.NoError(t, lightOpinionsEncoder.Close(), "error flushing light opinions")
				assert.NoError(t, lightOpinions.Close(), "error closing light opinions")

				darkOpinions, err := ioutil.TempFile("", fmt.Sprintf("fissile-%s-*.yml", testName))
				assert.NoError(t, err, "failed to create temporary dark opinions")
				require.NotNil(t, darkOpinions, "nil temporary dark opinions")
				defer os.Remove(darkOpinions.Name())
				darkOpinionsEncoder := yaml.NewEncoder(darkOpinions)
				assert.NoError(t, darkOpinionsEncoder.Encode(testData.DarkOpinions), "error encoding dark opinions")
				assert.NoError(t, darkOpinionsEncoder.Close(), "error flushing dark opinions")
				assert.NoError(t, darkOpinions.Close(), "error closing dark opinions")

				f := NewFissileApplication(".", ui)
				f.Options.RoleManifest = roleManifestPath
				f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
				f.Options.LightOpinions = lightOpinions.Name()
				f.Options.DarkOpinions = darkOpinions.Name()
				f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

				err = f.LoadManifest()
				assert.NoError(t, err)
				require.NotNil(t, f.Manifest, "error loading role manifest")

				errs := f.Validate()
				if len(testData.Errors) == 0 {
					assert.NoError(t, errs)
					return
				}

				actualErrors := errs.ErrorStrings()
				for _, expected := range testData.Errors {
					assert.Contains(t, actualErrors, expected)
				}
				assert.Len(t, actualErrors, len(testData.Errors))
			})
		}(roleManifestName[0 : len(roleManifestName)-len(".yml")])
	}

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

	allExpected := []string{
		`role-manifest 'not.a.hash.foo': Not found: "In any BOSH release"`,
		// `XXX`, // Trigger a fail which shows the contents of `actual`. Also template for new assertions.
	}
	for _, expected := range allExpected {
		assert.Contains(t, errs.ErrorStrings(), expected)
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
