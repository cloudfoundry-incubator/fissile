package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var jobPropertyMarshalTestCases = []struct {
	name     string
	property *JobProperty
	expected map[string]interface{}
}{
	{
		name: "simple",
		property: &JobProperty{
			Name:        "simple-property",
			Description: "A description",
			Default:     3,
			Job:         &Job{Name: "job-name"},
		},
		expected: map[string]interface{}{
			"name":        "simple-property",
			"description": "A description",
			"default":     3,
			"job":         "job-name",
		},
	},
	{
		name: "jobless",
		property: &JobProperty{
			Name:        "jobless-property",
			Description: "A different description",
			Default:     map[string]interface{}{"a": 1},
			Job:         nil,
		},
		expected: map[string]interface{}{
			"name":        "jobless-property",
			"description": "A different description",
			"default":     map[string]interface{}{"a": 1},
			"job":         "",
		},
	},
}

func TestJobPropertyMarshalYAML(t *testing.T) {
	for _, testCase := range jobPropertyMarshalTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert := assert.New(t)

			actual, err := testCase.property.MarshalYAML()
			if assert.NoError(err) {
				assert.Equal(testCase.expected, actual)
			}
		})
	}
}

func TestJobPropertyMarshalJSON(t *testing.T) {
	for _, testCase := range jobPropertyMarshalTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert := assert.New(t)

			expected, err := json.Marshal(testCase.expected)
			if !assert.NoError(err, "Failed to marshal expected JSON") {
				return
			}

			actual, err := testCase.property.MarshalJSON()
			if assert.NoError(err) {
				assert.JSONEq(string(expected), string(actual))
			}
		})
	}
}
