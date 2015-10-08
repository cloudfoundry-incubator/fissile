package builder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/hpcloud/fissile/scripts/dockerfiles"
	"github.com/pivotal-golang/archiver/extractor"
)

// BaseImageBuilder represents a builder of docker base images
type BaseImageBuilder struct {
	BaseImage string
}

// NewBaseImageBuilder creates a new BaseImageBuilder
func NewBaseImageBuilder(baseImage string) *BaseImageBuilder {
	return &BaseImageBuilder{
		BaseImage: baseImage,
	}
}

// CreateDockerfileDir generates a Dockerfile and assets in the targetDir
func (b *BaseImageBuilder) CreateDockerfileDir(targetDir, configginTarballPath string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil
	}

	dockerfilePath := filepath.Join(targetDir, "Dockerfile")
	dockerfileContents, err := b.generateDockerfile()
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(dockerfilePath, dockerfileContents, 0644); err != nil {
		return err
	}

	if err := b.unpackConfiggin(targetDir, configginTarballPath); err != nil {
		return err
	}

	monitrcPath := filepath.Join(targetDir, "monitrc.erb")
	monitrcContents, err := dockerfiles.Asset("scripts/dockerfiles/monitrc.erb")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(monitrcPath, monitrcContents, 0600); err != nil {
		return err
	}

	return nil
}

func (b *BaseImageBuilder) unpackConfiggin(targetDir, configginTarballPath string) error {

	configginDir := filepath.Join(targetDir, "configgin")

	if err := os.MkdirAll(configginDir, 0755); err != nil {
		return err
	}

	if err := extractor.NewTgz().Extract(configginTarballPath, configginDir); err != nil {
		return err
	}

	return nil
}

func (b *BaseImageBuilder) generateDockerfile() ([]byte, error) {
	asset, err := dockerfiles.Asset("scripts/dockerfiles/Dockerfile-base")
	if err != nil {
		return nil, err
	}

	dockerfileTemplate := template.New("Dockerfile-base")
	dockerfileTemplate, err = dockerfileTemplate.Parse(string(asset))
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	err = dockerfileTemplate.Execute(&output, b)
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func getBaseImageName(repository string) string {
	return fmt.Sprintf("%s:role-base", repository)
}
