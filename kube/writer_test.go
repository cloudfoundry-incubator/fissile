package kube

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func TestMain(m *testing.M) {
	retCode := m.Run()

	os.Exit(retCode)
}

func TestWriteOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	// Act
	config, err := GetYamlConfig(&apiv1.List{})

	// Assert
	assert.NoError(err)
	assert.Contains(config, "metadata")
}
