package kube

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	retCode := m.Run()

	os.Exit(retCode)
}

func TestWriteOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	// Act
	config, err := WriteConfig("foo")

	// Assert
	assert.NoError(err)
	assert.Contains(config, "foo")
}
