package builder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hpcloud/fissile/scripts/dockerfiles"
	"github.com/hpcloud/fissile/util"
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

// CreateDockerStream generates a Dockerfile and assets in the targetDir
func (b *BaseImageBuilder) CreateDockerStream(configginTarballPath string) (io.ReadCloser, <-chan error) {
	pipeReader, pipeWriter := io.Pipe()
	errors := make(chan error)

	go func() {
		defer close(errors)
		errors <- func() error {
			defer pipeWriter.Close()
			tarStream := tar.NewWriter(pipeWriter)
			defer tarStream.Close()

			// Generate dockerfile
			dockerfileContents, err := b.generateDockerfile()
			if err != nil {
				return err
			}
			err = util.WriteToTarStream(tarStream, dockerfileContents, tar.Header{
				Name: "Dockerfile",
			})
			if err != nil {
				return err
			}

			// Add configgin
			// The local function is to ensure we scope everything with access
			// to the (large) configgin binary so it can be freed early
			err = func() error {
				configginGzip, err := ioutil.ReadFile(configginTarballPath)
				if err != nil {
					return err
				}
				err = util.TargzIterate(configginTarballPath, bytes.NewReader(configginGzip), func(reader *tar.Reader, header *tar.Header) error {
					header.Name = filepath.Join("configgin", header.Name)
					if err = tarStream.WriteHeader(header); err != nil {
						return err
					}
					if _, err = io.Copy(tarStream, reader); err != nil {
						return err
					}
					return nil
				})
				return nil
			}()
			if err != nil {
				return err
			}

			// Add monitrc
			monitrcContents, err := dockerfiles.Asset("monitrc.erb")
			if err != nil {
				return err
			}
			err = util.WriteToTarStream(tarStream, monitrcContents, tar.Header{
				Name: "monitrc.erb",
				Mode: 0600,
			})
			if err != nil {
				return err
			}

			// Add rsyslog_conf
			for _, assetName := range dockerfiles.AssetNames() {
				if !strings.HasPrefix(assetName, "rsyslog_conf/") {
					continue
				}
				assetContents, err := dockerfiles.Asset(assetName)
				if err != nil {
					return err
				}
				err = util.WriteToTarStream(tarStream, assetContents, tar.Header{
					Name: assetName,
				})
				if err != nil {
					return err
				}
			}

			return nil
		}()
	}()

	return pipeReader, errors
}

func (b *BaseImageBuilder) generateDockerfile() ([]byte, error) {
	asset, err := dockerfiles.Asset("Dockerfile-base")
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

// GetBaseImageName generates a docker image name to be used as a role image base
func GetBaseImageName(repository, fissileVersion string) string {
	return util.SanitizeDockerName(fmt.Sprintf("%s-role-base:%s", repository, fissileVersion))
}
