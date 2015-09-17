package docker

import (
	"fmt"
	"io"
	"os"

	"github.com/fsouza/go-dockerclient"
)

type ImageManager interface {
	ListReleaseImages()
	FindBaseImage(imageName string)
	CompileInBaseContainer()
	CreateJobImage()
	UploadJobImage()
}

type FissileContainer struct {
	Container *docker.Container
	Stdin     io.Writer
	Stdout    io.Reader
	Stderr    io.Reader
}

type DockerImageManager struct {
	DockerEndpoint string

	client *docker.Client
}

func NewDockerImageManager(dockerEndpoint string) (*DockerImageManager, error) {
	manager := &DockerImageManager{
		DockerEndpoint: dockerEndpoint,
	}

	client, err := docker.NewClient(manager.DockerEndpoint)
	manager.client = client

	if err != nil {
		return nil, err
	}

	return manager, nil
}

func (d *DockerImageManager) ListReleaseImages() {

}

func (d *DockerImageManager) FindBaseImage(imageName string) (*docker.Image, error) {
	image, err := d.client.InspectImage(imageName)
	if err != nil {
		return nil, fmt.Errorf("Could not find base image %s: %s", imageName, err.Error())
	}

	return image, nil
}

func (d *DockerImageManager) CompileInBaseContainer() {

}

func (d *DockerImageManager) CreateJobImage() {

}

func (d *DockerImageManager) UploadJobImage() {

}

func (d *DockerImageManager) createCompilationContainer(containerName string, imageName string) (*FissileContainer, error) {
	cco := docker.CreateContainerOptions{
		Config: &docker.Config{
			AttachStdin:  false,
			AttachStdout: true,
			AttachStderr: true,
			Hostname:     "compiler",
			Domainname:   "fissile",
			Cmd:          []string{"ping", "google.com", "-c", "5"},
			WorkingDir:   "/",
			Image:        imageName,
		},
		HostConfig: &docker.HostConfig{
			Privileged: true,
		},
		Name: containerName,
	}

	container, err := d.client.CreateContainer(cco)
	if err != nil {
		return nil, err
	}

	err = d.client.StartContainer(container.ID, container.HostConfig)
	if err != nil {
		return nil, err
	}

	attached := make(chan struct{})
	//	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	//	stderrReader, stderrWriter := io.Pipe()

	//	stdinReader, stdinWriter := io.Pipe()
	//	stdoutReader, stdoutWriter := io.Pipe()
	//	stderrReader, stderrWriter := io.Pipe()

	go func() {
		err = d.client.AttachToContainer(docker.AttachToContainerOptions{
			Container: container.ID,

			InputStream:  os.Stdin, // stdinReader,
			OutputStream: stdoutWriter,
			ErrorStream:  os.Stderr, // stderrWriter,

			Stdin:       true,
			Stdout:      true,
			Stderr:      true,
			Stream:      true,
			RawTerminal: false,

			Success: attached,
		})

		if err != nil {
			panic(err)
		}
	}()

	attached <- <-attached

	//panic("Test")

	return &FissileContainer{
		Container: container,
		//		Stdin:     stdinWriter,
		Stdout: stdoutReader,
		//		Stderr:    stderrReader,
	}, nil
}
