package testhelpers

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

// Dedent converts Go-specific indentation (tabs) into proper YAML
// indentation.
func Dedent(x string) string {
	x = strings.Replace(x, "-\t", "-  ", -1)
	x = strings.Replace(x, "\t", "   ", -1)
	// fmt.Printf("YAML_________\n%s\n_____END\n", x)
	return x
}

// IsYAMLEqualString asserts that all items in the expected properties
// are in the actual properties, and vice versa. The expected
// properties are specified as YAML string. Go-specific indentation in
// the string (tabs) is replaced with proper YAML indentation.
func IsYAMLEqualString(assert *assert.Assertions, expected string, actual interface{}) bool {
	var expectedYAML interface{}
	if !assert.NoError(yaml.Unmarshal([]byte(Dedent(expected)),
		&expectedYAML)) {
		return false
	}
	return IsYAMLEqual(assert, expectedYAML, actual)
}

// IsYAMLEqual asserts that all items in the expected properties are
// in the actual properties, and vice versa.
func IsYAMLEqual(assert *assert.Assertions, expected, actual interface{}) bool {
	result := isYAMLSubsetInner(assert, expected, actual, nil)
	if !result {
		buf, err := yaml.Marshal(actual)
		if assert.NoError(err) {
			_, err := os.Stderr.Write(buf)
			assert.NoError(err)
		}
	}
	result = isYAMLSubsetInner(assert, actual, expected, nil)
	if !result {
		buf, err := yaml.Marshal(actual)
		if assert.NoError(err) {
			_, err := os.Stderr.Write(buf)
			assert.NoError(err)
		}
	}
	return result
}

// IsYAMLSubsetString asserts that all items in the expected
// properties are in the actual properties.  Note, the actual
// properties may contain more than expected. The expected properties
// are specified as YAML string. Go-specific indentation in the string
// (tabs) is replaced with proper YAML indentation.
func IsYAMLSubsetString(assert *assert.Assertions, expected string, actual interface{}) bool {
	var expectedYAML interface{}
	if !assert.NoError(yaml.Unmarshal([]byte(Dedent(expected)),
		&expectedYAML)) {
		return false
	}
	return IsYAMLSubset(assert, expectedYAML, actual)
}

// IsYAMLSubset asserts that all items in the expected properties are in the actual properties.
// Note, the actual properties may contain more than expected.
func IsYAMLSubset(assert *assert.Assertions, expected, actual interface{}) bool {
	result := isYAMLSubsetInner(assert, expected, actual, nil)
	if !result {
		buf, err := yaml.Marshal(actual)
		if assert.NoError(err) {
			_, err := os.Stderr.Write(buf)
			assert.NoError(err)
		}
	}
	return result
}

func isYAMLSubsetInner(assert *assert.Assertions, expected, actual interface{}, prefix []string) bool {
	yamlPath := strings.Join(prefix, ".")
	if yamlPath == "" {
		yamlPath = "<root>"
	}
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	actualType := "<nil>"
	if actualValue.IsValid() {
		actualType = fmt.Sprintf("%s", actualValue.Type())
	}

	switch expectedValue.Kind() {
	case reflect.Map:
		if !assert.Equal(reflect.Map, actualValue.Kind(), "expected YAML path %s to be a %s, but is actually %s", yamlPath, expectedValue.Type(), actualType) {
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
			// keyValue.String() does *not* return the contained value, but fmt has magic for reflect.Value types
			thisPrefix := append(prefix, fmt.Sprintf("%s", keyValue))
			if assert.True(actualValueValue.IsValid(), "missing key %s in YAML path %s", keyValue, yamlPath) {
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
		if !assert.Contains(allowedTypes, actualValue.Kind(), "expected YAML path %s to be a %s, but is actually %s", yamlPath, expectedValue.Type(), actualType) {
			return false
		}
		if !assert.Len(actualValue.Interface(), expectedValue.Len(), "expected slice at YAML path %s to have correct length", yamlPath) {
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
