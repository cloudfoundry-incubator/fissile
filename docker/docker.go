package docker

import (
	"github.com/fsouza/go-dockerclient"
)

type ImageManager interface {
	ListReleaseImages()
	FindBaseImage()
	CompileInBaseContainer()
	CreateJobImage()
	UploadJobImage()
}

type DockerImageManager struct {
	DockerEndpoint string
}

func (d *DockerImageManager) ListReleaseImages() {
	docker.NewClient("asd")
}

func (d *DockerImageManager) FindBaseImage() {

}

func (d *DockerImageManager) CompileInBaseContainer() {

}

func (d *DockerImageManager) CreateJobImage() {

}

func (d *DockerImageManager) UploadJobImage() {

}
