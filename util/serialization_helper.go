package util

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// The Marshaler interface is implemented by types which require custom
// marshalling support for dumping as JSON / YAML
type Marshaler interface {
	Marshal() (interface{}, error)
}

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

type marshalAdapter struct {
	object Marshaler
}

// MarshalJSON implements the encoding/json.Marshal interface
func (a *marshalAdapter) MarshalJSON() ([]byte, error) {
	result, err := a.object.Marshal()
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

// MarshalYAML implements the yaml.Marshal interface
func (a *marshalAdapter) MarshalYAML() (interface{}, error) {
	return a.object.Marshal()
}

// NewMarshalAdapter creates a new adapter for JSON/YAML marshalling
func NewMarshalAdapter(m Marshaler) interface{} {
	return &marshalAdapter{m}
}
