package builder

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	dockerImageEnvVar      = "FISSILE_TEST_DOCKER_IMAGE"
	defaultDockerTestImage = "ubuntu:14.04"
)

var dockerImageName string

func TestMain(m *testing.M) {
	dockerImageName = os.Getenv(dockerImageEnvVar)
	if dockerImageName == "" {
		dockerImageName = defaultDockerTestImage
	}

	retCode := m.Run()

	os.Exit(retCode)
}

func TestGenerateBaseImageDockerfile(t *testing.T) {
	assert := assert.New(t)

	baseImageBuilder := NewBaseImageBuilder("foo:bar")

	dockerfileContents, err := baseImageBuilder.generateDockerfile()
	assert.Nil(err)

	assert.NotNil(dockerfileContents)
	assert.Contains(string(dockerfileContents), "foo:bar")
}
