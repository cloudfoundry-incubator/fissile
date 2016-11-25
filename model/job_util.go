package model

import (
	"fmt"
	"strings"
)

// getKeyGrams converts a config key to its constituent parts
func getKeyGrams(key string) ([]string, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("BOSH config key cannot be empty")
	}
	return strings.Split(key, "."), nil
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

func getOpinionValue(parent map[interface{}]interface{}, keys []string) (interface{}, bool) {
	// fmt.Printf("QQQ: >> getOpinionValue(parent:%v, keys:%s)\n", parent, keys)
	var key string
	for _, key = range keys[:len(keys)-1] {
		child, ok := parent[key]
		// fmt.Printf("QQQ: Try key %s => %v\n", key, ok)
		if !ok {
			// fmt.Printf("QQQ: Failed to find %s in %v\n", key, parent)
			return nil, false
		}
		parent, ok = child.(map[interface{}]interface{})
		if !ok {
			// fmt.Printf("QQQ: Try type-checking child:%v\n", ok)
			return nil, false
		}
	}
	val, ok := parent[keys[len(keys)-1]]
	// fmt.Printf("QQQ: << %s|%v\n", val, ok)
	return val, ok
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
		result := []interface{}{}
		for _, elem := range valueSlice {
			result = append(result, valueToJSONable(elem))
		}
		return result
	}
	return value
}
