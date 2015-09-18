package docker

import (
	"fmt"
	"io"
	"sync"

	"github.com/fsouza/go-dockerclient"
)

const (
	ContainerInPath  = "/fissile-in"
	ContainerOutPath = "/fissile-out"
)

type ProcessOutStream func(io.Reader)

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

func (d *DockerImageManager) FindImage(imageName string) (*docker.Image, error) {
	image, err := d.client.InspectImage(imageName)
	if err != nil {
		return nil, fmt.Errorf("Could not find base image %s: %s", imageName, err.Error())
	}

	return image, nil
}

func (d *DockerImageManager) RemoveContainer(containerID string) error {
	return d.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerID,
		Force: true,
	})
}

func (d *DockerImageManager) RemoveImage(imageName string) error {
	return d.client.RemoveImage(imageName)
}

func (d *DockerImageManager) CreateImage(containerID string, repository string, tag string, message string, cmd []string) (*docker.Image, error) {
	cco := docker.CommitContainerOptions{
		Container:  containerID,
		Repository: repository,
		Tag:        tag,
		Author:     "fissile",
		Message:    message,
		Run: &docker.Config{
			Cmd: cmd,
		},
	}

	return d.client.CommitContainer(cco)
}

func (d *DockerImageManager) RunInContainer(containerName string, imageName string, cmd []string, inPath, outPath string, stdoutProcessor ProcessOutStream, stderrProcessor ProcessOutStream) (exitCode int, container *docker.Container, err error) {
	exitCode = -1

	cco := docker.CreateContainerOptions{
		Config: &docker.Config{
			AttachStdin:  false,
			AttachStdout: true,
			AttachStderr: true,
			Hostname:     "compiler",
			Domainname:   "fissile",
			Cmd:          cmd,
			WorkingDir:   "/",
			Image:        imageName,
		},
		HostConfig: &docker.HostConfig{
			Privileged: true,
			Binds:      []string{},
		},
		Name: containerName,
	}

	if inPath != "" {
		cco.HostConfig.Binds = append(cco.HostConfig.Binds, fmt.Sprintf("%s:%s:ro", inPath, ContainerInPath))
	}

	if outPath != "" {
		cco.HostConfig.Binds = append(cco.HostConfig.Binds, fmt.Sprintf("%s:%s", outPath, ContainerOutPath))
	}

	container, err = d.client.CreateContainer(cco)
	if err != nil {
		return -1, nil, err
	}

	attached := make(chan struct{})

	var stdoutReader, stderrReader io.ReadCloser
	var stdoutWriter, stderrWriter io.WriteCloser

	if stdoutProcessor != nil {
		stdoutReader, stdoutWriter = io.Pipe()
	}

	if stderrProcessor != nil {
		stderrReader, stderrWriter = io.Pipe()
	}

	go func() {
		err = d.client.AttachToContainer(docker.AttachToContainerOptions{
			Container: container.ID,

			InputStream:  nil,
			OutputStream: stdoutWriter,
			ErrorStream:  stderrWriter,

			Stdin:       false,
			Stdout:      stdoutProcessor != nil,
			Stderr:      stderrProcessor != nil,
			Stream:      true,
			RawTerminal: false,

			Success: attached,
		})
	}()

	attached <- <-attached

	err = d.client.StartContainer(container.ID, container.HostConfig)
	if err != nil {
		return -1, nil, err
	}

	var processorsGroup sync.WaitGroup

	if stdoutProcessor != nil {
		processorsGroup.Add(1)

		go func() {
			defer processorsGroup.Done()
			stdoutProcessor(stdoutReader)
		}()
	}

	if stderrProcessor != nil {
		processorsGroup.Add(1)

		go func() {
			defer processorsGroup.Done()
			stderrProcessor(stderrReader)
		}()
	}

	exitCode, err = d.client.WaitContainer(container.ID)
	if err != nil {
		return -1, nil, err
	}

	if stdoutWriter != nil {
		stdoutWriter.Close()
	}
	if stderrWriter != nil {
		stderrWriter.Close()
	}

	if stdoutReader != nil {
		stdoutReader.Close()
	}
	if stderrReader != nil {
		stderrReader.Close()
	}

	processorsGroup.Wait()

	return exitCode, container, nil
}
