package testhelpers

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/stretchr/testify/assert"
)

// IsYAMLSubset asserts that all items in the expected properties are in the actual properties
func IsYAMLSubset(assert *assert.Assertions, expected, actual interface{}) bool {
	return isYAMLSubsetInner(assert, expected, actual, nil)
}

func isYAMLSubsetInner(assert *assert.Assertions, expected, actual interface{}, prefix []string) bool {
	yamlPath := strings.Join(prefix, ".")
	if yamlPath == "" {
		yamlPath = "<root>"
	}
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	switch expectedValue.Kind() {
	case reflect.Map:
		if !assert.Equal(reflect.Map, actualValue.Kind(), "expected YAML path %s to be a %s, but is actually %s", yamlPath, expectedValue.Type(), actualValue.Type()) {
			return false
		}
		success := true
		for _, keyValue := range expectedValue.MapKeys() {
			var convertedKeyValue reflect.Value
			if actualValue.Type().Key().Kind() == reflect.String {
				convertedKeyValue = reflect.ValueOf(keyValue.Interface().(string))
			} else {
				convertedKeyValue = keyValue.Convert(actualValue.Type().Key())
			}
			expectedValueValue := expectedValue.MapIndex(keyValue)
			actualValueValue := actualValue.MapIndex(convertedKeyValue)
			thisPrefix := append(prefix, keyValue.String())
			if assert.True(actualValueValue.IsValid(), "missing key %s in YAML path %s", keyValue.String(), yamlPath) {
				if !isYAMLSubsetInner(assert, expectedValueValue.Interface(), actualValueValue.Interface(), thisPrefix) {
					success = false
				}
			} else {
				success = false
			}
		}
		return success
	case reflect.Array, reflect.Slice:
		allowedTypes := []reflect.Kind{reflect.Array, reflect.Slice}
		if !assert.Contains(allowedTypes, actualValue.Kind(), "expected YAML path %s to be a %s, but is actually %s", yamlPath, expectedValue.Type(), actualValue.Type()) {
			return false
		}
		if !assert.Equal(expectedValue.Len(), actualValue.Len(), "expected slice at YAML path %s to have correct length", yamlPath) {
			return false
		}
		success := true
		for i := 0; i < expectedValue.Len(); i++ {
			expectedValueValue := expectedValue.Index(i)
			actualValueValue := actualValue.Index(i)
			if !isYAMLSubsetInner(assert, expectedValueValue.Interface(), actualValueValue.Interface(), append(prefix, fmt.Sprintf("%d", i))) {
				success = false
			}
		}
		return success
	default:
		return assert.Equal(expected, actual, "unexpected value at YAML path %s", yamlPath)
	}
}
