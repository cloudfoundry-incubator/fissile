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

func TestJsonConfigWriterProvider(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	tmpDir, err := ioutil.TempDir("", "fissile-config-json-tests")
	assert.Nil(err)
	defer os.RemoveAll(tmpDir)
	outDir := filepath.Join(tmpDir, "store")

	builder := NewConfigStoreBuilder("foo", JSONProvider, opinionsFile, opinionsFileDark, outDir)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	err = builder.WriteBaseConfig([]*model.Release{release})
	assert.Nil(err)

	jsonPath := filepath.Join(outDir, "foo", "tor", "new_hostname.json")
	buf, err := ioutil.ReadFile(jsonPath)
	assert.Nil(err, "Failed to read output %s\n", jsonPath)

	var result map[string]interface{}
	err = json.Unmarshal(buf, &result)
	assert.Nil(err, "Error unmarshalling output")

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
		}
	}`), &expected)
	assert.Nil(err, "Failed to unmarshal expected data")
	assert.Equal(expected, result["parameters"], "Unexpected parameters")
}

func TestInitializeConfigJSON(t *testing.T) {
	assert := assert.New(t)

	config, err := initializeConfigJSON()
	assert.Nil(err)

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

func TestInsertConfig(t *testing.T) {
	assert := assert.New(t)
	var config, tempMap map[string]interface{}
	var err error
	var ok bool
	var buf []byte

	config = make(map[string]interface{})
	err = insertConfig(config, "hello.world", 123)
	assert.Nil(err)
	buf, err = json.Marshal(config)
	assert.Nil(err, "Error marshalling: %+v", err)
	err = json.Unmarshal(buf, &tempMap)
	assert.Nil(err, "Error unmarshalling: %+v", err)
	tempMap, ok = config["hello"].(map[string]interface{})
	assert.True(ok, "config does not have hello")
	assert.Equal(tempMap["world"], 123)

	config = make(map[string]interface{})
	err = insertConfig(config, "hello", map[interface{}]interface{}{
		"world": 123,
	})
	assert.Nil(err)
	buf, err = json.Marshal(config)
	assert.Nil(err, "Error marshalling: %+v", err)
	err = json.Unmarshal(buf, &tempMap)
	assert.Nil(err, "Error unmarshalling: %+v", err)
	tempMap, ok = config["hello"].(map[string]interface{})
	assert.True(ok, "config does not have hello")
	assert.Equal(tempMap["world"], 123)
}

func TestDeleteConfig(t *testing.T) {
	assert := assert.New(t)
	config := make(map[string]interface{})
	err := insertConfig(config, "hello.world", 123)
	assert.Nil(err)
	err = insertConfig(config, "hello.foo.bar", 111)
	assert.Nil(err)
	err = insertConfig(config, "hello.foo.quux", 222)
	assert.Nil(err)

	err = deleteConfig(config, "hello.world")
	assert.Nil(err)
	err = deleteConfig(config, "hello.foo.bar")
	assert.Nil(err)

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
