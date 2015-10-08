package builder

import (
	"io/ioutil"
	"os"
	"path/filepath"
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

func TestBaseImageCreateDockerfileDir(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	configginTarball := filepath.Join(workDir, "../test-assets/configgin/fake-configgin.tgz")

	targetDir, err := ioutil.TempDir("", "fissile-tests")
	assert.Nil(err)

	baseImageBuilder := NewBaseImageBuilder("foo:bar")

	err = baseImageBuilder.CreateDockerfileDir(targetDir, configginTarball)
	assert.Nil(err)

	dockerfilePath := filepath.Join(targetDir, "Dockerfile")
	contents, err := ioutil.ReadFile(dockerfilePath)
	assert.Nil(err)
	assert.Contains(string(contents), "foo:bar")

	configginPath := filepath.Join(targetDir, "configgin", "configgin")
	contents, err = ioutil.ReadFile(configginPath)
	assert.Nil(err)
	assert.Contains(string(contents), "exit 0")

	monitrcPath := filepath.Join(targetDir, "monitrc.erb")
	contents, err = ioutil.ReadFile(monitrcPath)
	assert.Nil(err)
	assert.Contains(string(contents), "hcf.monit.password")
}
