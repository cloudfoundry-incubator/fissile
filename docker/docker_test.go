package docker

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/fsouza/go-dockerclient"
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

func TestCreateCompilerContainer(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewDockerImageManager(dockerEndpoint)

	assert.Nil(err)

	container, err := dockerManager.createCompilationContainer(getTestName(), dockerImageName)

	defer func() {
		dockerManager.client.RemoveContainer(docker.RemoveContainerOptions{
			ID:    container.Container.ID,
			Force: true,
		})
	}()

	assert.Nil(err)
	assert.NotEmpty(container.Container.ID)

	go func() {
		io.Copy(os.Stdout, container.Stdout)
	}()

	//	buf := new(bytes.Buffer)
	//	buf.ReadFrom(container.Stdout)
	//	s := buf.String()
	//	panic(s)

	time.Sleep(10 * time.Second)
}

func getTestName() string {
	return fmt.Sprintf("fissile-test-%s", uuid.New())
}
