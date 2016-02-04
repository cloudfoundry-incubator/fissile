package configstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidBaseConfigProvider(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreBuilder("foo", "invalid-provider", "", "", "")
	err := confStore.WriteBaseConfig(nil)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid-provider", "Incorrect error")
}

func TestBOSHKeyToConsulPathConversion(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreBuilder("foo", "", "", "", "")

	boshKey := "this.is.a.bosh.key"

	consulPath, err := confStore.boshKeyToConsulPath(boshKey, DescriptionsStore)

	assert.Nil(err)

	assert.Equal("/foo/descriptions/this/is/a/bosh/key", consulPath)

}

func TestBOSHKeyToConsulPathConversionError(t *testing.T) {
	assert := assert.New(t)

	confStore := NewConfigStoreBuilder("foo", "", "", "", "")

	boshKey := ""

	_, err := confStore.boshKeyToConsulPath(boshKey, DescriptionsStore)

	assert.NotNil(err)
	assert.Contains(err.Error(), "BOSH config key cannot be empty")
}
