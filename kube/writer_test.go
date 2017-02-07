package kube

import (
	"bytes"
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
	assert := assert.New(t)

	yamlConfig := bytes.Buffer{}
	err := WriteYamlConfig(&apiv1.List{}, &yamlConfig)

	assert.NoError(err)
	assert.Contains(yamlConfig.String(), "metadata")
}
