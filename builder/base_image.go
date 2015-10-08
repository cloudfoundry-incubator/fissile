package builder

import (
	"bytes"
	"text/template"

	"github.com/hpcloud/fissile/scripts/dockerfiles"
)

type BaseImageBuilder struct {
	BaseImage string
}

func NewBaseImageBuilder(baseImage string) *BaseImageBuilder {
	return &BaseImageBuilder{
		BaseImage: baseImage,
	}
}

func (b *BaseImageBuilder) CreateDockerfileDir(targetDir string) error {

	return nil
}

func (b *BaseImageBuilder) generateDockerfile() ([]byte, error) {
	asset, err := dockerfiles.Asset("scripts/dockerfiles/Dockerfile-base")
	if err != nil {
		return nil, err
	}

	dockerfileTemplate := template.New("t")
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
