package builder

import (
	"archive/tar"
	"io"
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

func TestBaseImageCreateDockerStream(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)
	configginTarball := filepath.Join(workDir, "../test-assets/configgin/fake-configgin.tgz")

	baseImageBuilder := NewBaseImageBuilder("foo:bar")
	tarStream, errors := baseImageBuilder.CreateDockerStream(configginTarball)

	testFunctions := map[string]func([]byte){
		"Dockerfile": func(rawContents []byte) {
			assert.Contains(string(rawContents), "foo:bar")
		},
		"configgin/configgin": func(rawContents []byte) {
			assert.Contains(string(rawContents), "exit 0")
		},
		"monitrc.erb": func(rawContents []byte) {
			assert.Contains(string(rawContents), "hcf.monit.password")
		},
	}

	tarReader := tar.NewReader(tarStream)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if !assert.NoError(err) {
			break
		}
		if tester, ok := testFunctions[header.Name]; ok {
			actual, err := ioutil.ReadAll(tarReader)
			assert.NoError(err)
			tester(actual)
			delete(testFunctions, header.Name)
		}
	}
	assert.Empty(testFunctions, "Missing files in tar stream")
	assert.NoError(<-errors)
}
