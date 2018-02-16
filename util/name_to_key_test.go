package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertNameToKey(t *testing.T) {
	t.Parallel()

	input := "APP_PASSPHRASE"

	output := ConvertNameToKey(input)

	assert.Equal(t, "app-passphrase", output)
}
