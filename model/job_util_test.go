package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueToJSONable(t *testing.T) {
	assert := assert.New(t)

	assert.Nil(valueToJSONable(nil))
	assert.Equal(123, valueToJSONable(123))
	assert.Equal("hello", valueToJSONable("hello"))

	assert.Equal(map[string]interface{}{
		"hello": map[string]interface{}{
			"world": 0,
		},
	}, valueToJSONable(map[interface{}]interface{}{
		"hello": map[interface{}]interface{}{
			"world": 0,
		},
	}))

	assert.Equal([]interface{}{
		0: map[string]interface{}{
			"hello": 123,
		},
	}, valueToJSONable([]interface{}{
		0: map[interface{}]interface{}{
			"hello": 123,
		},
	}))
}

func TestInsertConfig(t *testing.T) {
	assert := assert.New(t)
	var config, tempMap map[string]interface{}
	var err error
	var ok bool
	var buf []byte

	config = make(map[string]interface{})
	err = insertConfig(config, "hello.world", 123)
	assert.NoError(err)
	buf, err = json.Marshal(config)
	assert.NoError(err, "Error marshalling")
	err = json.Unmarshal(buf, &tempMap)
	assert.NoError(err, "Error unmarshalling")
	tempMap, ok = config["hello"].(map[string]interface{})
	assert.True(ok, "config does not have hello")
	assert.Equal(tempMap["world"], 123)

	config = make(map[string]interface{})
	err = insertConfig(config, "hello", map[interface{}]interface{}{
		"world": 123,
	})
	assert.NoError(err)
	buf, err = json.Marshal(config)
	assert.NoError(err, "Error marshalling")
	err = json.Unmarshal(buf, &tempMap)
	assert.NoError(err, "Error unmarshalling")
	tempMap, ok = config["hello"].(map[string]interface{})
	assert.True(ok, "config does not have hello")
	assert.Equal(tempMap["world"], 123)
}
