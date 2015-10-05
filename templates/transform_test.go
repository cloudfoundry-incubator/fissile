package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hpcloud/fissile/scripts/templates"
)

func TestTransformUnmarshal(t *testing.T) {
	assert := assert.New(t)

	asset, err := templates.Asset("scripts/templates/transformations.yml")
	assert.Nil(err)
	transformer, err := NewTransformer(string(asset))

	assert.Nil(err)
	assert.NotNil(transformer)
}

func TestTransformPropertyFunctionOk(t *testing.T) {
	assert := assert.New(t)

	asset, err := templates.Asset("scripts/templates/transformations.yml")
	assert.Nil(err)
	transformer, err := NewTransformer(string(asset))

	assert.Nil(err)
	assert.NotNil(transformer)

	result, err := transformer.Transform(`properties.router.logrotate.freq_min`)
	assert.Nil(err)
	assert.Equal(`{{index . "router" "logrotate" "freq_min" }}`, result)
}
