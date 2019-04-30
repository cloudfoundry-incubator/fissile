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

// TestValidation runs through some data-driven tests to check that validation
// errors are reported correctly.
//
// It goes through all the YAML files in the
// .../test-assets/role-manifests/app/validation/ directory, each of which is
// expected to be an extended role manifest.  In addition to the normal
// contents, it should also have top level key "expected_errors" which contains
// a list of strings, one each per expected error.  The ordering is ignored.
// The extended role manifest may also have "light_opinions" and "dark_opinions"
// keys; each of those can be used as the light (resp. dark) opinions files.
// They will most likely contain the "properties" key at the next level.
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
					assert.Empty(t, errs)
					return
				}

				actualErrors := errs.ErrorStrings()
				sort.Strings(actualErrors)
				assert.Equal(t, testData.Errors, actualErrors, "unexpected validation errors")
			})
		}(roleManifestName[0 : len(roleManifestName)-len(".yml")])
	}

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
