package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"code.google.com/p/go-uuid/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	dockerEndpointEnvVar      = "FISSILE_TEST_DOCKER_ENDPOINT"
	defaultDockerTestEndpoint = "unix:///var/run/docker.sock"
	dockerImageEnvVar         = "FISSILE_TEST_DOCKER_IMAGE"
	defaultDockerTestImage    = "ubuntu:14.04"
)

var dockerEndpoint string
var dockerImageName string

func TestMain(m *testing.M) {
	dockerEndpoint = os.Getenv(dockerEndpointEnvVar)
	if dockerEndpoint == "" {
		dockerEndpoint = defaultDockerTestEndpoint
	}

	dockerImageName = os.Getenv(dockerImageEnvVar)
	if dockerImageName == "" {
		dockerImageName = defaultDockerTestImage
	}

	retCode := m.Run()

	os.Exit(retCode)
}

func TestFindImageOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)
	assert.Nil(err)

	image, err := dockerManager.FindBaseImage(dockerImageName)

	assert.Nil(err)
	assert.NotEmpty(image.ID)
}

func TestShowImageNotOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)
	assert.Nil(err)

	_, err = dockerManager.FindBaseImage(uuid.New())

	assert.NotNil(err)
	assert.Contains(err.Error(), "Could not find base image")
}

func TestRunInContainer(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.runInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ping", "127.0.0.1", "-c", "1"},
		"",
		"",
		func(stdout io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stdout)
			output = buf.String()
		},
		nil,
	)

	assert.Nil(err)
	assert.Equal(0, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.removeContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerStderr(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.runInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ping", "-foo"},
		"",
		"",
		nil,
		func(stderr io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stderr)
			output = buf.String()
		},
	)

	assert.Nil(err)
	assert.Equal(2, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.removeContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.runInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ls", "/fissile-in"},
		"/",
		"",
		func(stdout io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stdout)
			output = buf.String()
		},
		nil,
	)

	assert.Nil(err)
	assert.Equal(0, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.removeContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithReadOnlyInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	exitCode, container, err := dockerManager.runInContainer(
		getTestName(),
		dockerImageName,
		[]string{"touch", "/fissile-in/fissile-test.txt"},
		"/",
		"",
		nil,
		nil,
	)

	assert.Nil(err)
	assert.NotEqual(0, exitCode)

	err = dockerManager.removeContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.runInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ls", "/fissile-out"},
		"",
		"/tmp",
		func(stdout io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stdout)
			output = buf.String()
		},
		nil,
	)

	assert.Nil(err)
	assert.Equal(0, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.removeContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithWritableOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	exitCode, container, err := dockerManager.runInContainer(
		getTestName(),
		dockerImageName,
		[]string{"touch", "/fissile-out/fissile-test.txt"},
		"",
		"/tmp",
		nil,
		nil,
	)

	assert.Nil(err)
	assert.Equal(0, exitCode)

	err = dockerManager.removeContainer(container.ID)
	assert.Nil(err)
}

func TestCreateImageOk(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	exitCode, container, err := dockerManager.runInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ping", "127.0.0.1", "-c", "1"},
		"",
		"",
		nil,
		nil,
	)

	assert.Nil(err)
	assert.Equal(0, exitCode)

	testRepo := getTestName()
	testTag := getTestName()

	image, err := dockerManager.createImage(
		container.ID,
		testRepo,
		testTag,
		"fissile-test",
		[]string{"ping", "127.0.0.1", "-c", "1"},
	)

	assert.Nil(err)
	assert.NotEmpty(image.ID)

	err = dockerManager.removeContainer(container.ID)
	assert.Nil(err)

	err = dockerManager.removeImage(fmt.Sprintf("%s:%s", testRepo, testTag))
	assert.Nil(err)
}

func getTestName() string {
	return fmt.Sprintf("fissile-test-%s", uuid.New())
}
