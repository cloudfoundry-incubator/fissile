package util

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// JSONMarshal marshals an arbitrary map to JSON; this only exists because
// JSON.Marshal insists on having maps that have string (and not interface{})
// keys
func JSONMarshal(input interface{}) ([]byte, error) {
	converted, err := jsonMarshalHelper(input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(converted)
}

type jsonMarshalError struct {
	msg  string
	keys []string
}

func (e jsonMarshalError) Error() string {
	return fmt.Sprintf("Error marshalling JSON: Failed to convert keys in path %s: %s",
		strings.Join(e.keys, "."), e.msg)
}

// jsonMarshalHelper converts a map from having interface{} keys to string keys
func jsonMarshalHelper(input interface{}) (interface{}, *jsonMarshalError) {
	value := reflect.ValueOf(input)
	switch value.Kind() {
	case reflect.Map:
		result := make(map[string]interface{})
		for _, keyValue := range value.MapKeys() {
			keyInterface := keyValue.Interface()
			keyString, ok := keyInterface.(string)
			if !ok {
				return nil, &jsonMarshalError{msg: fmt.Sprintf("Invalid key %#v", keyInterface)}
			}
			valueInterface := value.MapIndex(keyValue).Interface()
			convertedValue, err := jsonMarshalHelper(valueInterface)
			if err != nil {
				err.keys = append([]string{keyString}, err.keys...)
				return nil, err
			}
			result[keyString] = convertedValue
		}

		return result, nil
	case reflect.Array, reflect.Slice:
		var result []interface{}
		for i := 0; i < value.Len(); i++ {
			element, err := jsonMarshalHelper(value.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			result = append(result, element)
		}
		return result, nil
	default:
		return input, nil
	}
}

// JSONMergeBlobs merges two JSON-compatible maps.  It is an error if the two
// maps have children that have the same path to the values.
func JSONMergeBlobs(dest, src map[string]interface{}) error {
	return jsonMergeBlobsHelper(dest, src, []string{})
}

func assignBlob(dest map[string]interface{}, srcKey string, srcValue interface{}) error {
	// No destValue for key
	if _, ok := srcValue.(map[string]interface{}); ok {
		// If the source is a hash, we need to copy it. Otherwise a leaf of the source
		// tree could be modified while it's sitting in the dest tree, and that would
		// mutate the source tree.  Easy way to copy a tree in go is to json marshal/unmarshal
		jsonBytes, err := json.Marshal(srcValue)
		if err != nil {
			return err
		}
		newValue := map[string]interface{}{}
		err = json.Unmarshal(jsonBytes, &newValue)
		if err != nil {
			return err
		}
		dest[srcKey] = newValue
	} else {
		dest[srcKey] = srcValue
	}
	return nil
}

// Preconditions:
// destValue != nil
// srcValue != nil
// destValue is not a map[string]itf

// Return an error if src is a map, or if one is an array, and the other isn't.
func typeCheck(destValue, srcValue interface{}, srcKey string, path []string) error {
	srcType := reflect.TypeOf(srcValue)
	destType := reflect.TypeOf(destValue)
	_, srcIsMap := srcValue.(map[string]interface{})
	if srcIsMap {
		return fmt.Errorf("Invalid merge near %s: cannot merge non-map %v (%v) with map %v (%v)",
			strings.Join(append(path, srcKey), "."), destValue, destType, srcValue, srcType)
	}
	_, srcIsArray := srcValue.([]interface{})
	_, destIsArray := destValue.([]interface{})
	if srcIsArray != destIsArray {
		return fmt.Errorf("Invalid merge near %s: cannot merge array/non-array %v (%v) with %v (%v)",
			strings.Join(append(path, srcKey), "."), destValue, destType, srcValue, srcType)
	}
	return nil
}

func jsonMergeBlobsHelper(dest, src map[string]interface{}, path []string) error {
	for srcKey, srcValue := range src {
		destValue, ok := dest[srcKey]
		if !ok {
			err := assignBlob(dest, srcKey, srcValue)
			if err != nil {
				return err
			}
			continue
		}
		destMap, ok := destValue.(map[string]interface{})
		if !ok {
			// destValue is _not_ a map; not sure about new value yet
			// if either the src or dest is a nil use the other
			// if src is a map, complain
			if srcValue == nil {
				// Ignore src nils
				continue
			}
			if destValue == nil {
				// Copy the src value for the dest
				err := assignBlob(dest, srcKey, srcValue)
				if err != nil {
					return err
				}
				continue
			}
			err := typeCheck(destValue, srcValue, srcKey, path)
			if err != nil {
				return err
			}
			err = assignBlob(dest, srcKey, srcValue)
			if err != nil {
				return err
			}
			continue
		}
		srcMap, ok := srcValue.(map[string]interface{})
		if !ok {
			if srcValue == nil {
				continue
			}
			return fmt.Errorf("Invalid merge near %s: was %v, new value is %v",
				strings.Join(append(path, srcKey), "."), destValue, srcValue)
		}
		if err := jsonMergeBlobsHelper(destMap, srcMap, append(path, srcKey)); err != nil {
			return err
		}
	}
	return nil
}
