package configstore

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
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

	builder := NewConfigStoreBuilder(JSONProvider, opinionsFile, opinionsFileDark, outDir)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	err = builder.WriteBaseConfig(rolesManifest)
	assert.NoError(err)

	jsonPath := filepath.Join(outDir, "myrole", "tor.json")

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
		   "tor": {
            "client_keys": null,
            "hashed_control_password": null,
            "hostname": "localhost",
            "private_key": null
        }
	}`), &expected)
	assert.NoError(err, "Failed to unmarshal expected data")

	assert.Equal(expected, result["properties"], "Unexpected properties")
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

	_, ok = config["properties"].(map[string]interface{})
	assert.True(ok, "Properties should be a map")

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

	deleteConfig(config, []string{"hello", "world"}, nil)
	deleteConfig(config, []string{"hello", "foo", "bar"}, nil)
	deleteConfig(config, []string{"hello", "does", "not", "exist"}, nil)

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

func TestConfigMapDifference(t *testing.T) {
	assert := assert.New(t)

	var leftMap map[string]interface{}
	err := json.Unmarshal([]byte(`{
	    "toplevel": "value",
	    "key": {
	        "secondary": "value2",
	        "empty": {
	            "removed": "value3"
	        },
	        "extra": 4
	    },
	    "also_removed": "yes"
	}`), &leftMap)
	assert.NoError(err)

	var rightMap map[interface{}]interface{}
	err = yaml.Unmarshal([]byte(strings.Replace(`
	key:
	    empty:
	        removed: true
	    extra: yes
	    not_in_config: yay
	also_removed: please
	`, "\t", "    ", -1)), &rightMap)
	assert.NoError(err)

	err = configMapDifference(leftMap, rightMap, []string{})
	assert.NoError(err)

	var expected map[string]interface{}
	err = json.Unmarshal([]byte(`{
	    "toplevel": "value",
	    "key": {
	        "secondary": "value2",
	        "empty": {}
	    }
	}`), &expected)
	assert.NoError(err)

	assert.Equal(expected, leftMap)
}

func TestConfigMapDifferenceError(t *testing.T) {
	assert := assert.New(t)

	config := map[string]interface{}{
		"a": map[string]interface{}{
			"b": 1234,
		},
	}

	difference := map[interface{}]interface{}{
		"a": map[interface{}]interface{}{
			"b": map[interface{}]interface{}{
				"c": 1234,
			},
		},
	}

	err := configMapDifference(config, difference, []string{"q"})
	assert.EqualError(err, "Attempting to descend into dark opinions key q.a.b which is not a hash in the base configuration")
}

func TestJSONConfigWriterProvider_MultipleBOSHReleases(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	tmpDir, err := ioutil.TempDir("", "fissile-config-json-tests-mutliple-bosh-releases")
	assert.NoError(err)
	defer os.RemoveAll(tmpDir)
	outDir := filepath.Join(tmpDir, "store")

	builder := NewConfigStoreBuilder(JSONProvider, opinionsFile, opinionsFileDark, outDir)

	var releases []*model.Release
	for _, releaseName := range []string{"tor-boshrelease", "ntp-release"} {
		releasePath := filepath.Join(workDir, "../test-assets", releaseName)
		releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
		release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
		assert.NoError(err)
		releases = append(releases, release)
	}

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/multiple-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, releases)
	assert.NoError(err)

	err = builder.WriteBaseConfig(rolesManifest)
	assert.NoError(err)

	var result map[string]interface{}

	// Verify the properties are correctly reflected into the three different JSONs
	jsonPath := filepath.Join(outDir, "myrole", "ntpd.json")
	buf, err := ioutil.ReadFile(jsonPath)
	if !assert.NoError(err, "Failed to read output %s\n", jsonPath) {
		return
	}
	err = json.Unmarshal(buf, &result)
	if !assert.NoError(err, "Error unmarshalling output") {
		return
	}

	assert.Equal("myrole", result["job"].(map[string]interface{})["name"])

	templates := result["job"].(map[string]interface{})["templates"]
	assert.Contains(templates, map[string]interface{}{"name": "tor"})
	assert.Contains(templates, map[string]interface{}{"name": "new_hostname"})
	assert.Contains(templates, map[string]interface{}{"name": "ntpd"})
	assert.Len(templates, 3)

	properties := result["properties"].(map[string]interface{})
	torProp, ok := properties["tor"].(map[string]interface{})
	if assert.True(ok) {
		privateKey, ok := torProp["private_key"]
		if assert.True(ok) {
			assert.NotNil(privateKey)
		}
	}

	// Check that the new_hostname job didn't pick up superfluous properties
	jsonPath = filepath.Join(outDir, "myrole", "new_hostname.json")
	buf, err = ioutil.ReadFile(jsonPath)
	if !assert.NoError(err, "Failed to read output %s\n", jsonPath) {
		return
	}
	err = json.Unmarshal(buf, &result)
	if !assert.NoError(err, "Error unmarshalling output") {
		return
	}
	assert.Equal("myrole", result["job"].(map[string]interface{})["name"])
	templates = result["job"].(map[string]interface{})["templates"]
	assert.Contains(templates, map[string]interface{}{"name": "tor"})
	assert.Contains(templates, map[string]interface{}{"name": "new_hostname"})
	assert.Len(templates, 3)
	nullProperties := result["properties"]
	if assert.NotNil(nullProperties, "new_hostname.properties shouldn't be nil") {
		npFixed, ok := nullProperties.(map[string]interface{})
		if assert.True(ok, "Expected nullProperties to be a hash, got %v", nullProperties) {
			for k, v := range npFixed {
				assert.Fail(fmt.Sprintf("new_hostname.properties should be empty, have: key:%s, val:%v", k, v))
			}
		}
	}

	// Check that the tor job has correct settings
	jsonPath = filepath.Join(outDir, "myrole", "tor.json")

	buf, err = ioutil.ReadFile(jsonPath)
	if !assert.NoError(err, "Failed to read output %s\n", jsonPath) {
		return
	}

	err = json.Unmarshal(buf, &result)
	if !assert.NoError(err, "Error unmarshalling output") {
		return
	}
	assert.Equal("myrole", result["job"].(map[string]interface{})["name"])
	templates = result["job"].(map[string]interface{})["templates"]
	assert.Contains(templates, map[string]interface{}{"name": "tor"})
	assert.Contains(templates, map[string]interface{}{"name": "new_hostname"})
	assert.Len(templates, 3)
	properties = result["properties"].(map[string]interface{})

	actualJSON, err := json.Marshal(properties)
	if assert.NoError(err) {
		assert.JSONEq(`{
			"tor": {
				"client_keys": null,
				"hashed_control_password": null,
				"hostname": "localhost",
				"private_key": null
			}
		}`, string(actualJSON), "Unexpected properties")
	}
}
