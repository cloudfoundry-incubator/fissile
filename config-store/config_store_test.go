package config_store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBOSHKeyToConsulPathConversion(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreManager("foo")

	boshKey := "this.is.a.bosh.key"

	consulPath, err := confStore.boshKeyToConsulPath(boshKey)

	assert.Nil(err)

	assert.Equal("/foo/this/is/a/bosh/key", consulPath)

}

func TestBOSHKeyToConsulPathConversionError(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreManager("foo")

	boshKey := ""

	_, err := confStore.boshKeyToConsulPath(boshKey)

	assert.NotNil(err)
	assert.Contains(err.Error(), "BOSH config key cannot be empty")
}

func TestConsulPathToBOSHKeyConversion(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreManager("foo")

	boshKey := "/foo/this/is/a/consul/path"

	boshKey, err := confStore.consulPathToBOSHKey(boshKey)

	assert.Nil(err)
	assert.Equal("this.is.a.consul.path", boshKey)
}

func TestConsulPathToBOSHKeyConversionBadPrefix(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreManager("foo")

	boshKey := "/bar/this/is/a/consul/path"

	boshKey, err := confStore.consulPathToBOSHKey(boshKey)

	assert.NotNil(err)
	assert.Contains(err.Error(), "does not have the correct prefix")
}

func TestConsulPathToBOSHKeyConversionBadKey(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreManager("foo")

	boshKey := "/foo/"

	boshKey, err := confStore.consulPathToBOSHKey(boshKey)

	assert.NotNil(err)
	assert.Contains(err.Error(), "is not a valid consul key")
}
