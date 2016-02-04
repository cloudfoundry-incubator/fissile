package configstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetKeyGrams(t *testing.T) {
	assert := assert.New(t)

	result, err := getKeyGrams("")
	assert.Error(err, "Getting key grams for nothing should have an error")

	result, err = getKeyGrams("hello")
	assert.Nil(err)
	assert.Equal([]string{"hello"}, result)

	result, err = getKeyGrams("hello.world")
	assert.Nil(err)
	assert.Equal([]string{"hello", "world"}, result)
}

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
