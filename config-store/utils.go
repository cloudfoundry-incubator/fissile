package configstore

import (
	"fmt"
	"strings"
)

// getKeyGrams converts a config key to its constituent parts
func getKeyGrams(key string) ([]string, error) {
	keyGrams := strings.FieldsFunc(key, func(c rune) bool { return c == '.' })
	if len(keyGrams) == 0 {
		return nil, fmt.Errorf("BOSH config key cannot be empty")
	}

	return keyGrams, nil
}

// valueToJSONable ensures that the given value can be converted to JSON
func valueToJSONable(value interface{}) interface{} {
	if valueMap, ok := value.(map[interface{}]interface{}); ok {
		result := make(map[string]interface{})
		for k, v := range valueMap {
			result[k.(string)] = valueToJSONable(v)
		}
		return result
	}
	if valueSlice, ok := value.([]interface{}); ok {
		var result []interface{}
		for _, elem := range valueSlice {
			result = append(result, valueToJSONable(elem))
		}
		return result
	}
	return value
}
