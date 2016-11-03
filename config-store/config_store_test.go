package configstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestInvalidBaseConfigProvider(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	outputPath, err := ioutil.TempDir("", "fissile-config-store-tests")
	assert.NoError(err)
	defer os.RemoveAll(outputPath)

	lightOpinionsPath := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	darkOpinionsPath := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	confStore := NewConfigStoreBuilder("invalid-provider", lightOpinionsPath, darkOpinionsPath, outputPath)
	err = confStore.WriteBaseConfig(rolesManifest)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid-provider", "Incorrect error")
}

func TestBOSHKeyToConsulPathConversion(t *testing.T) {
	assert := assert.New(t)

	boshKey := "this.is.a.bosh.key"

	consulPath, err := BoshKeyToConsulPath(boshKey, DescriptionsStore)

	assert.NoError(err)

	assert.Equal("/descriptions/this/is/a/bosh/key", consulPath)

}

func TestBOSHKeyToConsulPathConversionError(t *testing.T) {
	assert := assert.New(t)

	boshKey := ""

	_, err := BoshKeyToConsulPath(boshKey, DescriptionsStore)

	assert.Error(err)
	assert.Contains(err.Error(), "BOSH config key cannot be empty")
}

// getKeys is a helper method to get all the keys in a nested JSON structure, as BOSH-style dot-separated names
func getKeys(props map[string]map[string]interface{}) []string {
	var results []string
	var innerFunc func(props map[string]interface{}, prefix []string)

	innerFunc = func(props map[string]interface{}, prefix []string) {
		for key, value := range props {
			fullKey := append(prefix, key)
			if valueStringMap, ok := value.(map[string]interface{}); ok {
				innerFunc(valueStringMap, fullKey)
			} else {
				results = append(results, strings.Join(fullKey, "."))
			}
		}
	}

	for _, releaseProps := range props {
		innerFunc(releaseProps, []string{})
	}
	return results
}

func TestGetAllPropertiesForRoleManifest(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	allProps, namesWithoutDefaults, err := getAllPropertiesForRoleManifest(rolesManifest)
	assert.NoError(err)

	keys := getKeys(allProps)
	sort.Strings(keys)
	assert.Equal([]string{
		"tor.client_keys",
		"tor.hashed_control_password",
		"tor.hostname",
		"tor.private_key",
	}, keys)

	var noDefaultKeys []string
	for _, namesWithoutDefaultsForRole := range namesWithoutDefaults {
		for key := range namesWithoutDefaultsForRole {
			noDefaultKeys = append(noDefaultKeys, key)
		}
	}
	sort.Strings(noDefaultKeys)
	assert.Equal([]string{
		"tor.client_keys",
		"tor.hashed_control_password",
		"tor.private_key",
	}, noDefaultKeys)
}

func TestCheckKeysInProperties(t *testing.T) {
	testSamples := []struct {
		name                 string   // A name for this test sample
		opinions             string   // The opinions to load
		properties           string   // The properties to load
		namesWithoutDefaults []string // Names without default values
		expectedErrors       []string // Substrings that should be in errors
		containsWarnings     []string // Warnings expected (each on its own line)
		notContainsWarnings  []string // Warnings not expected (each on its own line)
	}{
		{
			name: "expected warnings",
			opinions: `
				properties:
					parent:
						extra-key: 3
						nested-extra-key:
							child-key: 1
						no-defaults:
							new-child: 2
			`,
			properties: `{
				"parent": {
					"missing-key": 4,
					"no-defaults": {}
				}
			}`,
			namesWithoutDefaults: []string{
				"parent.no-defaults",
			},
			containsWarnings: []string{
				"parent.extra-key",
				"parent.nested-extra-key",
			},
			notContainsWarnings: []string{
				"parent.missing-key",
				"parent.nested-extra-key.child-key",
				"parent.empty.new-child",
			},
		},
		{
			name: "extra toplevel opinions",
			opinions: `
				properties:
					parent:
						child: 1
			`,
			properties: `{
				"parent-is-missing": {}
			}`,
			expectedErrors: []string{
				"extra top level keys",
				"parent",
			},
		},
		{
			name: "missing toplevel properties ok",
			opinions: `
				properties: {}
			`,
			properties: `{
				"extra-top-level-property": {
					"key": 1
				}
			}`,
		},
	}

	assert := assert.New(t)

	for _, testSample := range testSamples {
		var opinions, props map[string]interface{}
		var err error

		err = yaml.Unmarshal([]byte(strings.Replace(testSample.opinions, "\t", "  ", -1)), &opinions)
		opinionsOk := assert.NoError(err, "error parsing opinions for test `%s'", testSample.name)

		err = json.Unmarshal([]byte(testSample.properties), &props)
		propertiesOk := assert.NoError(err, "error parsing properties for test `%s'", testSample.name)
		if !opinionsOk || !propertiesOk {
			// No need to continue the rest of the test if the samples are bad
			continue
		}

		namesWithoutDefaultsForRole := make(map[string]struct{}, len(testSample.namesWithoutDefaults))
		for _, name := range testSample.namesWithoutDefaults {
			namesWithoutDefaultsForRole[name] = struct{}{}
		}
		namesWithoutDefaults := map[string]map[string]struct{}{"R:J": namesWithoutDefaultsForRole}

		warningBuf := bytes.Buffer{}
		err = checkKeysInProperties(opinions, map[string]map[string]interface{}{"R:J": props}, namesWithoutDefaults, testSample.name, &warningBuf)

		if len(testSample.expectedErrors) > 0 {
			// We expect errors back from the function
			if assert.Error(err, "did not receive expected error while running test `%s'", testSample.name) {
				for _, expectedError := range testSample.expectedErrors {
					assert.Contains(err.Error(), expectedError, "missing expected error while running test `%s'", testSample.name)
				}
			}
			assert.Empty(warningBuf.String(), "got warnings when errors were expected while running test `%s'", testSample.name)
			continue
		}

		assert.NoError(err, "got unexpected error while running test `%s'", testSample.name)

		if len(testSample.containsWarnings) == 0 {
			assert.Empty(warningBuf.String(), "test `%s' should have no warnings", testSample.name)
			continue
		}

		lines := strings.Split(warningBuf.String(), "\n")
		var warnings []string
		for _, line := range lines[1:] {
			warnings = append(warnings, strings.TrimSpace(line))
		}
		assert.Contains(
			lines[0],
			fmt.Sprintf("%s opinions", testSample.name),
			"did not find opinion name while running test `%s'",
			testSample.name,
		)
		for _, warning := range testSample.containsWarnings {
			assert.Contains(warnings, warning, "did not find expected warning in test `%s'", testSample.name)
		}
		for _, warning := range testSample.notContainsWarnings {
			assert.NotContains(warnings, warning, "found unexpected warning in test `%s'", testSample.name)
		}
	}
}
