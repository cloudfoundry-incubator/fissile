package builder

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
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

func TestBaseImageNewDockerPopulator(t *testing.T) {
	assert := assert.New(t)

	baseImageBuilder := NewBaseImageBuilder("foo:bar")
	buffer := &bytes.Buffer{}
	tarPopulator := baseImageBuilder.NewDockerPopulator()
	assert.NoError(tarPopulator(tar.NewWriter(buffer)))

	testFunctions := map[string]func([]byte){
		"Dockerfile": func(rawContents []byte) {
			assert.Contains(string(rawContents), "foo:bar")
		},
		"configgin/configgin": func(rawContents []byte) {
			assert.Contains(string(rawContents), "BUNDLE_GEMFILE")
		},
		"monitrc.erb": func(rawContents []byte) {
			assert.Contains(string(rawContents), "hcf.monit.password")
		},
	}

	tarReader := tar.NewReader(buffer)
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
}

func TestBaseImageNewDockerPopulatorWithError(t *testing.T) {
	assert := assert.New(t)

	tarPopulator := NewBaseImageBuilder("foo:bar").NewDockerPopulator()
	// We give it a closed writer, and ensure that the error bubbles up
	pipeReader, pipeWriter, err := os.Pipe()
	assert.NoError(err)
	assert.NoError(pipeWriter.Close())
	assert.NoError(pipeReader.Close())
	err = tarPopulator(tar.NewWriter(pipeWriter))
	assert.Error(err)
}
