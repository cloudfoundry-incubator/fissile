package builder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	configstore "github.com/hpcloud/fissile/config-store"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/dockerfiles"
	"github.com/hpcloud/fissile/util"
	"github.com/hpcloud/termui"
)

// PackagesImageBuilder represents a builder of the shared packages layer docker image
type PackagesImageBuilder struct {
	repository           string
	compiledPackagesPath string
	targetPath           string
	fissileVersion       string
	ui                   *termui.UI
}

// NewPackagesImageBuilder creates a new PackagesImageBuilder
func NewPackagesImageBuilder(repository, compiledPackagesPath, targetPath, fissileVersion string, ui *termui.UI) (*PackagesImageBuilder, error) {
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, err
	}
	return &PackagesImageBuilder{
		repository:           repository,
		compiledPackagesPath: compiledPackagesPath,
		targetPath:           targetPath,
		fissileVersion:       fissileVersion,
		ui:                   ui,
	}, nil
}

// tarWalker is a helper to copy files into a tar stream
type tarWalker struct {
	stream *tar.Writer // The stream to copy the files into
	root   string      // The base directory on disk where the walking started
	prefix string      // The prefix in the tar file the names should have
}

func (w *tarWalker) walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	if (info.Mode() & os.ModeSymlink) != 0 {
		linkname, err := os.Readlink(path)
		if err != nil {
			return err
		}
		header.Linkname = linkname
	}
	relPath, err := filepath.Rel(w.root, path)
	if err != nil {
		return err
	}
	header.Name = filepath.Join(w.prefix, relPath)
	if err := w.stream.WriteHeader(header); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err = io.CopyN(w.stream, file, info.Size()); err != nil {
			return err
		}
	}
	return nil
}

// CreatePackagesDockerStream generates a tar stream containing the docker context to build the packages layer image with
func (p *PackagesImageBuilder) CreatePackagesDockerStream(roleManifest *model.RoleManifest, lightManifestPath, darkManifestPath string) (io.ReadCloser, <-chan error, error) {
	if len(roleManifest.Roles) == 0 {
		return nil, nil, fmt.Errorf("No roles to build")
	}

	pipeReader, pipeWriter := io.Pipe()
	errors := make(chan error)

	go func() {
		defer close(errors)
		defer pipeWriter.Close()
		tarStream := tar.NewWriter(pipeWriter)
		defer tarStream.Close()

		// Generate configuration
		specDir, err := ioutil.TempDir("", "fissile-spec-dir")
		if err != nil {
			errors <- err
			return
		}
		defer os.RemoveAll(specDir)
		configStore := configstore.NewConfigStoreBuilder(
			configstore.JSONProvider,
			lightManifestPath,
			darkManifestPath,
			specDir,
		)

		if err := configStore.WriteBaseConfig(roleManifest); err != nil {
			errors <- fmt.Errorf("Error writing base config: %s", err.Error())
			return
		}
		walker := &tarWalker{stream: tarStream, root: specDir, prefix: "specs"}
		if err := filepath.Walk(specDir, walker.walk); err != nil {
			errors <- err
			return
		}

		// Collect compiled packages
		foundFingerprints := make(map[string]bool)
		for _, role := range roleManifest.Roles {
			for _, job := range role.Jobs {
				for _, pkg := range job.Packages {
					if _, ok := foundFingerprints[pkg.Fingerprint]; ok {
						// Package has already been found (possibly due to a different role)
						continue
					}
					walker := &tarWalker{
						stream: tarStream,
						root:   pkg.GetPackageCompiledDir(p.compiledPackagesPath),
						prefix: filepath.Join("packages-src", pkg.Fingerprint),
					}
					err := filepath.Walk(walker.root, walker.walk)
					if err != nil {
						errors <- err
						return
					}
					foundFingerprints[pkg.Fingerprint] = true
				}
			}
		}

		// Generate dockerfile
		dockerfile := bytes.Buffer{}
		baseImageName := GetBaseImageName(p.repository, p.fissileVersion)
		if err := p.generateDockerfile(baseImageName, &dockerfile); err != nil {
			errors <- err
			return
		}
		header := tar.Header{
			Name:     "Dockerfile",
			Mode:     0644,
			Size:     int64(dockerfile.Len()),
			Typeflag: tar.TypeReg,
		}
		if err := tarStream.WriteHeader(&header); err != nil {
			errors <- err
			return
		}
		if _, err := tarStream.Write(dockerfile.Bytes()); err != nil {
			errors <- err
			return
		}
	}()
	return pipeReader, errors, nil
}

// generateDockerfile builds a docker file for the shared packages layer.
func (p *PackagesImageBuilder) generateDockerfile(baseImage string, outputFile io.Writer) error {
	context := map[string]interface{}{
		"base_image": baseImage,
	}
	asset, err := dockerfiles.Asset("Dockerfile-packages")
	if err != nil {
		return err
	}

	dockerfileTemplate := template.New("Dockerfile")
	dockerfileTemplate, err = dockerfileTemplate.Parse(string(asset))
	if err != nil {
		return err
	}

	if err := dockerfileTemplate.Execute(outputFile, context); err != nil {
		return err
	}

	return nil
}

// GetRolePackageImageName generates a docker image name for the amalgamation for a role image
func (p *PackagesImageBuilder) GetRolePackageImageName(roleManifest *model.RoleManifest) string {
	return util.SanitizeDockerName(fmt.Sprintf("%s-role-packages:%s",
		p.repository,
		roleManifest.GetRoleManifestDevPackageVersion(p.fissileVersion),
	))
}
