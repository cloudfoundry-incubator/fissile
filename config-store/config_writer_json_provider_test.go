package configstore

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/stretchr/testify/assert"
)

func TestJSONConfigWriterProvider(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	tmpDir, err := ioutil.TempDir("", "fissile-config-json-tests")
	assert.NoError(err)
	defer os.RemoveAll(tmpDir)
	outDir := filepath.Join(tmpDir, "store")

	builder := NewConfigStoreBuilder("foo", JSONProvider, opinionsFile, opinionsFileDark, outDir)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := model.NewRelease(releasePath)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	err = builder.WriteBaseConfig(rolesManifest)
	assert.NoError(err)

	jsonPath := filepath.Join(outDir, "foo", "myrole", "new_hostname.json")
	buf, err := ioutil.ReadFile(jsonPath)
	if !assert.NoError(err, "Failed to read output %s\n", jsonPath) {
		return
	}

	var result map[string]interface{}
	err = json.Unmarshal(buf, &result)
	if !assert.NoError(err, "Error unmarshalling output") {
		return
	}

	assert.Equal("myrole", result["job"].(map[string]interface{})["name"])

	templates := result["job"].(map[string]interface{})["templates"]
	assert.Contains(templates, map[string]interface{}{"name": "tor"})
	assert.Contains(templates, map[string]interface{}{"name": "new_hostname"})
	assert.Len(templates, 2)

	var expected map[string]interface{}
	err = json.Unmarshal([]byte(`{
		"test": {
			"property": {
				"name": {
					"array": [
						"one",
						"two",
						"three"
					],
					"key": "value",
					"number": 123
				}
			}
		},
		"has": {
			"opinion": "this is an opinion"
		},
		"cc": {
			"app_events": {
				"cutoff_age_in_days": 31
			},
			"droplets": {}
		}
	}`), &expected)
	assert.NoError(err, "Failed to unmarshal expected data")
	// We don't care about the "tor" branch of the parameters; that comes from upstream
	// All the things we want to test are the custom stuff we added.
	delete(result["parameters"].(map[string]interface{}), "tor")
	assert.Equal(expected, result["parameters"], "Unexpected parameters")

}

func TestInitializeConfigJSON(t *testing.T) {
	assert := assert.New(t)

	config, err := initializeConfigJSON()
	assert.NoError(err)

	jobConfig, ok := config["job"].(map[string]interface{})
	assert.True(ok, "Job config should be a map with string keys")
	assert.NotNil(jobConfig["templates"])
	_, ok = jobConfig["templates"].([]interface{})
	assert.True(ok, "Job templates should be an array")

	_, ok = config["parameters"].(map[string]interface{})
	assert.True(ok, "Parameters should be a map")

	networks, ok := config["networks"].(map[string]interface{})
	assert.True(ok, "Networks should be a map")
	_, ok = networks["default"].(map[string]interface{})
	assert.True(ok, "Network defaults should be a map")
}

func TestDeleteConfig(t *testing.T) {
	assert := assert.New(t)
	config := make(map[string]interface{})
	err := insertConfig(config, "hello.world", 123)
	assert.NoError(err)
	err = insertConfig(config, "hello.foo.bar", 111)
	assert.NoError(err)
	err = insertConfig(config, "hello.foo.quux", 222)
	assert.NoError(err)

	err = deleteConfig(config, "hello.world")
	assert.NoError(err)
	err = deleteConfig(config, "hello.foo.bar")
	assert.NoError(err)
	err = deleteConfig(config, "hello.does.not.exist")
	assert.IsType(&errConfigNotExist{}, err)

	hello, ok := config["hello"].(map[string]interface{})
	assert.True(ok)
	_, ok = hello["world"]
	assert.False(ok)
	foo, ok := hello["foo"].(map[string]interface{})
	assert.True(ok)
	_, ok = foo["bar"]
	assert.False(ok)
	_, ok = foo["quux"]
	assert.True(ok)
}
