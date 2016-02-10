package configstore

import (
	"encoding/json"
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

// insertConfig adds a configuration value into the configuration map
func insertConfig(config map[string]interface{}, name string, value interface{}) error {
	keyPieces, err := getKeyGrams(name)
	if err != nil {
		return err
	}

	parent := config
	for _, key := range keyPieces[:len(keyPieces)-1] {
		if child, ok := parent[key].(map[string]interface{}); ok {
			parent = child
		} else {
			child = make(map[string]interface{})
			parent[key] = child
			parent = child
		}
	}
	parent[keyPieces[len(keyPieces)-1]] = valueToJSONable(value)
	return nil
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

// deepCopy make a deep copy of a JSON-able map
func deepCopy(in map[string]interface{}) (map[string]interface{}, error) {
	buf, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(buf, &result); err != nil {
		return nil, err
	}
	return result, nil
}
