package builder

import (
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
	shutil "github.com/termie/go-shutil"
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

// CreatePackagesDockerBuildDir generates a Dockerfile and assets for the shared packages layer and returns a path to the dir
func (p *PackagesImageBuilder) CreatePackagesDockerBuildDir(roleManifest *model.RoleManifest, lightManifestPath, darkManifestPath string) (string, error) {
	if len(roleManifest.Roles) == 0 {
		return "", fmt.Errorf("No roles to build")
	}

	succeeded := false
	buildDir, err := ioutil.TempDir(p.targetPath, "role-packages")
	if err != nil {
		return "", err
	}
	defer func() {
		if !succeeded {
			os.RemoveAll(buildDir)
		}
	}()
	rootDir := filepath.Join(buildDir, "root")

	// Generate dockerfile
	dockerfile, err := os.Create(filepath.Join(buildDir, "Dockerfile"))
	if err != nil {
		return "", err
	}
	defer dockerfile.Close()
	baseImageName := GetBaseImageName(p.repository, p.fissileVersion)
	if err := p.generateDockerfile(baseImageName, dockerfile); err != nil {
		return "", err
	}

	// Generate configuration
	p.ui.Println("Generating configuration JSON specs ...")
	configStore := configstore.NewConfigStoreBuilder(
		configstore.JSONProvider,
		lightManifestPath,
		darkManifestPath,
		filepath.Join(rootDir, "opt/hcf/specs"),
	)

	if err := configStore.WriteBaseConfig(roleManifest); err != nil {
		return "", fmt.Errorf("Error writing base config: %s", err.Error())
	}

	// Copy packages
	p.ui.Println("Copying compiled packages...")
	copiedFingerprints := make(map[string]bool)
	for _, role := range roleManifest.Roles {
		for _, job := range role.Jobs {
			for _, pkg := range job.Packages {
				if _, ok := copiedFingerprints[pkg.Fingerprint]; ok {
					// Package has already been copied (possibly due to a different role)
					continue
				}

				compiledDir := pkg.GetPackageCompiledDir(p.compiledPackagesPath)
				if err := util.ValidatePath(compiledDir, true, fmt.Sprintf("compiled dir for package %s", pkg.Name)); err != nil {
					return "", err
				}

				packageDir := filepath.Join(rootDir, "var/vcap/packages-src", pkg.Fingerprint)
				err = shutil.CopyTree(
					compiledDir,
					packageDir,
					&shutil.CopyTreeOptions{
						Symlinks:               true,
						Ignore:                 nil,
						CopyFunction:           shutil.Copy,
						IgnoreDanglingSymlinks: false,
					},
				)
				if err != nil {
					return "", err
				}

				copiedFingerprints[pkg.Fingerprint] = true
			}
		}
	}

	succeeded = true
	return buildDir, nil
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
