package docker

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
)

type ImageManager interface {
	ListReleaseImages()
	FindBaseImage(imageName string)
	CompileInBaseContainer()
	CreateJobImage()
	UploadJobImage()
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
