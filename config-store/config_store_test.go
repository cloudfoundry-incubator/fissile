package config_store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBOSHKeyToConsulPathConversion(t *testing.T) {
	assert := assert.New(t)

	confStore, err := NewConfigStoreManager("foo", "http://foo:5800/dc1")
	assert.Nil(err)

	boshKey := "this.is.a.bosh.key"

	consulPath := confStore.boshKeyToConsulPath(boshKey)

	assert.Equal("/foo/this/is/a/bosh/key", consulPath)

}
