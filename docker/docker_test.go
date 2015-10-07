package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"code.google.com/p/go-uuid/uuid"
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

func TestFindImageOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.Nil(err)

	image, err := dockerManager.FindImage(dockerImageName)

	assert.Nil(err)
	assert.NotEmpty(image.ID)
}

func TestShowImageNotOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.Nil(err)

	_, err = dockerManager.FindImage(uuid.New())

	assert.NotNil(err)
	assert.Contains(err.Error(), "Could not find base image")
}

func TestRunInContainer(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
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

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerStderr(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
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

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ls", ContainerInPath},
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

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithReadOnlyInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"touch", filepath.Join(ContainerInPath, "fissile-test.txt")},
		"/",
		"",
		nil,
		nil,
	)

	assert.Nil(err)
	assert.NotEqual(0, exitCode)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ls", ContainerOutPath},
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

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithWritableOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"touch", filepath.Join(ContainerOutPath, "fissile-test.txt")},
		"",
		"/tmp",
		nil,
		nil,
	)

	assert.Nil(err)
	assert.Equal(0, exitCode)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestCreateImageOk(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	exitCode, container, err := dockerManager.RunInContainer(
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

	image, err := dockerManager.CreateImage(
		container.ID,
		testRepo,
		testTag,
		"fissile-test",
		[]string{"ping", "127.0.0.1", "-c", "1"},
	)

	assert.Nil(err)
	assert.NotEmpty(image.ID)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)

	err = dockerManager.RemoveImage(fmt.Sprintf("%s:%s", testRepo, testTag))
	assert.Nil(err)
}

func getTestName() string {
	return fmt.Sprintf("fissile-test-%s", uuid.New())
}
