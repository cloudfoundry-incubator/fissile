package util

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/SUSE/fissile/testhelpers"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

type jsonHelpInputTestData struct {
	name   string
	yaml   string
	json   string
	errMsg string
}

func TestJSONHelperValidInput(t *testing.T) {
	assert := assert.New(t)

	testDataList := []jsonHelpInputTestData{
		jsonHelpInputTestData{
			name: "Simple number input",
			yaml: `1`,
			json: `1`,
		},
		jsonHelpInputTestData{
			name: "Simple map",
			yaml: `a: 1`,
			json: `{"a": 1}`,
		},
		jsonHelpInputTestData{
			name: "Nested map",
			yaml: `a: { b: c }`,
			json: `{"a": {"b": "c"}}`,
		},
		jsonHelpInputTestData{
			name: "Map in slice",
			yaml: `[ { a: b } ]`,
			json: `[ {"a": "b" } ]`,
		},
		jsonHelpInputTestData{
			name:   "Map with non-string keys",
			yaml:   `1: 2`,
			errMsg: `Failed to convert keys in path : Invalid key 1`,
		},
		jsonHelpInputTestData{
			name:   "Nested map with non-string keys",
			yaml:   `a: { b: { 1: 2 } }`,
			errMsg: `Failed to convert keys in path a.b: Invalid key 1`,
		},
	}

	for _, testData := range testDataList {
		var unmarshaled interface{}
		err := yaml.Unmarshal([]byte(testData.yaml), &unmarshaled)
		if !assert.NoError(err, "Failed to unmarshal YAML data for test sample %s", testData.name) {
			continue
		}
		result, err := JSONMarshal(unmarshaled)
		if testData.errMsg != "" {
			assert.Error(err, fmt.Sprintf("Expected test sample %s to result in an error", testData.name))
			assert.Contains(err.Error(), testData.errMsg, "Error message did not contain expected string for test sample %s", testData.name)
		} else {
			if assert.NoError(err, "Unexpected error for test sample %s", testData.name) {
				assert.JSONEq(testData.json, string(result), "Unexpected result for test sample %s", testData.name)
			}
		}
	}
}

type dummyMarshaler struct {
	obj interface{}
}

func (m *dummyMarshaler) Marshal() (interface{}, error) {
	return m.obj, nil
}

func TestMarshalAdapter(t *testing.T) {
	assert := assert.New(t)
	adapter := NewMarshalAdapter(&dummyMarshaler{3})

	if jsonResult, err := json.Marshal(adapter); assert.NoError(err) {
		assert.JSONEq("3", string(jsonResult))
	}
	if yamlResult, err := yaml.Marshal(adapter); assert.NoError(err) {
		var unmarshalled interface{}
		if assert.NoError(yaml.Unmarshal(yamlResult, &unmarshalled)) {
			testhelpers.IsYAMLSubset(assert, 3, unmarshalled)
		}
	}
}
