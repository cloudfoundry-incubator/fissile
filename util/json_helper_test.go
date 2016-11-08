package util

import (
	"encoding/json"
	"testing"

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
			assert.Error(err, "Exepected test sample %s to result in an error", testData.name)
			assert.Contains(err.Error(), testData.errMsg, "Error message did not contain expected string for test sample %s", testData.name)
		} else {
			if assert.NoError(err, "Unexpected error for test sample %s", testData.name) {
				assert.JSONEq(testData.json, string(result), "Unexpected result for test sample %s", testData.name)
			}
		}
	}
}

type jsonMergeBlobsTestData struct {
	name     string
	source   string
	dest     string
	expected string
	errMsg   string
}

func TestMergeJSONBlobs(t *testing.T) {
	assert := assert.New(t)

	testDataList := []jsonMergeBlobsTestData{
		jsonMergeBlobsTestData{
			name:     "Simple copy",
			source:   `{"a": 1}`,
			dest:     `{}`,
			expected: `{"a": 1}`,
		},
		jsonMergeBlobsTestData{
			name:     "Simple merge",
			source:   `{"a": 1}`,
			dest:     `{"b": 2}`,
			expected: `{"a": 1, "b": 2}`,
		},
		jsonMergeBlobsTestData{
			name:     "No input",
			source:   `{}`,
			dest:     `{"b": 2}`,
			expected: `{"b": 2}`,
		},
		jsonMergeBlobsTestData{
			name:     "Merging hashes",
			source:   `{"root": {"a": 1}}`,
			dest:     `{"root": {"b": 2}}`,
			expected: `{"root": {"a": 1, "b": 2}}`,
		},
		jsonMergeBlobsTestData{
			name:     "Ignore equal values",
			source:   `{"root": {"child": 1}}`,
			dest:     `{"root": {"child": 1}}`,
			expected: `{"root": {"child": 1}}`,
		},
		jsonMergeBlobsTestData{
			name:     "With same types, ignore subsequent values",
			source:   `{"root": {"child": 1}}`,
			dest:     `{"root": {"child": 2}}`,
			expected: `{"root": {"child": 1}}`,
		},
		jsonMergeBlobsTestData{
			name:     "Ignore equal arrays",
			source:   `{"root": {"child": [1, 2]}}`,
			dest:     `{"root": {"child": [1, 2]}}`,
			expected: `{"root": {"child": [1, 2]}}`,
		},
		jsonMergeBlobsTestData{
			name:     "With same types (as arrays), ignore subsequent values",
			source:   `{"root": {"child": [1]}}`,
			dest:     `{"root": {"child": [2]}}`,
			expected: `{"root": {"child": [1]}}`,
		},
		jsonMergeBlobsTestData{
			name:   "Error trying to merge different types",
			source: `{"root": {"child": [1]}}`,
			dest:   `{"root": {"child": 2}}`,
			errMsg: "near root.child: cannot merge 2 with [1]",
		},
	}

	for _, testData := range testDataList {
		var source, dest map[string]interface{}
		assert.NoError(json.Unmarshal([]byte(testData.source), &source),
			"Error unmarshaling source data for test sample %s", testData.name)
		assert.NoError(json.Unmarshal([]byte(testData.dest), &dest),
			"Error unmarshaling destination data for test sample %s", testData.name)
		err := JSONMergeBlobs(dest, source)
		if testData.errMsg != "" {
			if assert.Error(err, "Expected test sample %s to result in an error", testData.name) {
				assert.Contains(err.Error(), testData.errMsg,
					"Error message did not contain expected string for test sample %s", testData.name)
			}
		} else if assert.NoError(err, "Unexpected error for test sample %s", testData.name) {
			result, err := json.Marshal(dest)
			assert.NoError(err, "Unexpected error marshalling result")
			assert.JSONEq(testData.expected, string(result),
				"Unexpected result for test sample %s", testData.name)
		}
	}
}
