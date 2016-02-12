package configstore

import (
	"bytes"
	"encoding/json"
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

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := model.NewRelease(releasePath)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	confStore := NewConfigStoreBuilder("foo", "invalid-provider", lightOpinionsPath, darkOpinionsPath, outputPath)
	err = confStore.WriteBaseConfig(rolesManifest)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid-provider", "Incorrect error")
}

func TestBOSHKeyToConsulPathConversion(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreBuilder("foo", "", "", "", "")

	boshKey := "this.is.a.bosh.key"

	consulPath, err := confStore.boshKeyToConsulPath(boshKey, DescriptionsStore)

	assert.NoError(err)

	assert.Equal("/foo/descriptions/this/is/a/bosh/key", consulPath)

}

func TestBOSHKeyToConsulPathConversionError(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreBuilder("foo", "", "", "", "")

	boshKey := ""

	_, err := confStore.boshKeyToConsulPath(boshKey, DescriptionsStore)

	assert.Error(err)
	assert.Contains(err.Error(), "BOSH config key cannot be empty")
}

// getKeys is a helper method to get all the keys in a nested JSON structure, as BOSH-style dot-separated names
func getKeys(props map[string]interface{}) []string {
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

	innerFunc(props, []string{})
	return results
}

func TestGetAllPropertiesForRoleManifest(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := model.NewRelease(releasePath)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	allProps, err := getAllPropertiesForRoleManifest(rolesManifest)
	assert.NoError(err)

	keys := getKeys(allProps)
	sort.Strings(keys)
	assert.Equal([]string{
		"cc.app_events.cutoff_age_in_days",
		"cc.droplets.droplet_directory_key",
		"has.dark-opinion",
		"has.opinion",
		"test.property.name.array",
		"test.property.name.key",
		"test.property.name.number",
		"tor.client_keys",
		"tor.hashed_control_password",
		"tor.hostname",
		"tor.private_key",
	}, keys)
}

func TestCheckKeysInProperties(t *testing.T) {
	assert := assert.New(t)

	var opinions, props map[string]interface{}

	err := yaml.Unmarshal([]byte(strings.Replace(`
		properties:
			parent:
				extra-key: 3
				nested-extra-key:
					child-key: 1
	`, "\t", "  ", -1)), &opinions)
	assert.NoError(err)

	err = json.Unmarshal([]byte(`{
		"parent": {
			"missing-key": 4
		}
	}`), &props)
	assert.NoError(err)

	warningBuf := bytes.Buffer{}
	err = checkKeysInProperties(opinions, props, "testing", &warningBuf)
	assert.NoError(err)
	assert.Contains(warningBuf.String(), "testing opinions")
	assert.Contains(warningBuf.String(), "parent.extra-key")
	assert.Contains(warningBuf.String(), "parent.nested-extra-key")
	assert.NotContains(warningBuf.String(), "parent.missing-key")
	assert.NotContains(warningBuf.String(), "parent.nested-extra-key.child-key")

	warningBuf = bytes.Buffer{}
	opinions = map[string]interface{}{
		"properties": make(map[interface{}]interface{}),
	}
	err = checkKeysInProperties(opinions, props, "testing-two", &warningBuf)
	assert.NoError(err)
	assert.Empty(warningBuf.String())
}
