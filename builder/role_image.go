package builder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/dockerfiles"
	"github.com/hpcloud/fissile/util"

	"github.com/fatih/color"
	"github.com/termie/go-shutil"
)

const (
	binPrefix = "bin"
)

// RoleImageBuilder represents a builder of docker role images
type RoleImageBuilder struct {
	repository               string
	compiledPackagesPath     string
	targetPath               string
	defaultConsulAddress     string
	defaultConfigStorePrefix string
	version                  string
	fissileVersion           string
}

// NewRoleImageBuilder creates a new RoleImageBuilder
func NewRoleImageBuilder(repository, compiledPackagesPath, targetPath, defaultConsulAddress, defaultConfigStorePrefix, version, fissileVersion string) *RoleImageBuilder {
	return &RoleImageBuilder{
		repository:               repository,
		compiledPackagesPath:     compiledPackagesPath,
		targetPath:               targetPath,
		defaultConsulAddress:     defaultConsulAddress,
		defaultConfigStorePrefix: defaultConfigStorePrefix,
		version:                  version,
		fissileVersion:           fissileVersion,
	}
}

// CreateDockerfileDir generates a Dockerfile and assets in the targetDir and returns a path to the dir
func (r *RoleImageBuilder) CreateDockerfileDir(role *model.Role) (string, error) {
	if len(role.Jobs) == 0 {
		return "", fmt.Errorf("Error - role %s has 0 jobs", role.Name)
	}

	// Create a dir for the role
	roleDir := filepath.Join(r.targetPath, role.Name)
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		return "", err
	}

	// Write out license files
	if license := &role.Jobs[0].Release.License; license.Contents != nil {
		err := ioutil.WriteFile(filepath.Join(roleDir, license.Filename), license.Contents, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write out license file: %v", err)
		}
	}
	if notice := role.Jobs[0].Release.Notice; notice.Contents != nil {
		err := ioutil.WriteFile(filepath.Join(roleDir, notice.Filename), notice.Contents, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write out notice file: %v", err)
		}
	}

	// Copy compiled packages
	packagesDir := filepath.Join(roleDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		return "", err
	}
	packageSet := map[string]string{}
	for _, job := range role.Jobs {
		for _, pkg := range job.Packages {
			// Only copy packages that we haven't copied before
			if _, ok := packageSet[pkg.Name]; ok == false {
				packageSet[pkg.Name] = pkg.Fingerprint
			} else {
				if pkg.Fingerprint != packageSet[pkg.Name] {
					log.Printf("WARNING: duplicate package %s. Using package with fingerprint %s.\n",
						color.CyanString(pkg.Name), color.RedString(packageSet[pkg.Name]))
				}

				continue
			}

			compiledDir := filepath.Join(r.compiledPackagesPath, pkg.Name, pkg.Fingerprint, "compiled")
			if err := util.ValidatePath(compiledDir, true, fmt.Sprintf("compiled dir for package %s", pkg.Name)); err != nil {
				return "", err
			}

			packageDir := filepath.Join(packagesDir, pkg.Name)
			if role.Jobs[0].Release.Dev {

				if err := os.Symlink(compiledDir, packageDir); err != nil {
					return "", err
				}
			} else {
				if err := shutil.CopyTree(
					compiledDir,
					packageDir,
					&shutil.CopyTreeOptions{
						Symlinks:               true,
						Ignore:                 nil,
						CopyFunction:           shutil.Copy,
						IgnoreDanglingSymlinks: false},
				); err != nil {
					return "", err
				}
			}
		}
	}

	// Copy jobs templates and monit
	jobsDir := filepath.Join(roleDir, "jobs")
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		return "", err
	}
	for _, job := range role.Jobs {
		jobDir, err := job.Extract(jobsDir)
		if err != nil {
			return "", err
		}

		jobManifestFile := filepath.Join(jobDir, "job.MF")
		if err := os.Remove(jobManifestFile); err != nil {
			return "", err
		}

		for _, template := range job.Templates {
			templatePath := filepath.Join(jobDir, "templates", template.SourcePath)
			if strings.HasPrefix(template.DestinationPath, fmt.Sprintf("%s%c", binPrefix, os.PathSeparator)) {
				os.Chmod(templatePath, 0755)
			} else {
				os.Chmod(templatePath, 0644)
			}
		}
	}

	// Copy role startup scripts
	startupDir := filepath.Join(roleDir, "role-startup")
	if err := os.MkdirAll(startupDir, 0755); err != nil {
		return "", err
	}
	for script, sourceScriptPath := range role.GetScriptPaths() {
		destScriptPath := filepath.Join(startupDir, script)
		destDir := filepath.Dir(destScriptPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return "", err

		}
		if err := shutil.CopyFile(sourceScriptPath, destScriptPath, true); err != nil {
			return "", err
		}
	}

	// Generate run script
	runScriptContents, err := r.generateRunScript(role)
	if err != nil {
		return "", err
	}
	runScriptPath := filepath.Join(roleDir, "run.sh")
	if err := ioutil.WriteFile(runScriptPath, runScriptContents, 0744); err != nil {
		return "", err
	}

	// Generate Dockerfile
	dockerfileContents, err := r.generateDockerfile(role)
	if err != nil {
		return "", err
	}
	dockerfilePath := filepath.Join(roleDir, "Dockerfile")
	if err := ioutil.WriteFile(dockerfilePath, dockerfileContents, 0644); err != nil {
		return "", err
	}

	return roleDir, nil
}

func (r *RoleImageBuilder) generateRunScript(role *model.Role) ([]byte, error) {
	asset, err := dockerfiles.Asset("scripts/dockerfiles/run.sh")
	if err != nil {
		return nil, err
	}

	runScriptTemplate := template.New("role-runscript")
	context := map[string]interface{}{
		"default_consul_address":      r.defaultConsulAddress,
		"default_config_store_prefix": r.defaultConfigStorePrefix,
		"role": role,
	}
	runScriptTemplate, err = runScriptTemplate.Parse(string(asset))
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	err = runScriptTemplate.Execute(&output, context)
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func (r *RoleImageBuilder) generateDockerfile(role *model.Role) ([]byte, error) {
	baseImage := GetBaseImageName(r.repository, r.fissileVersion)

	asset, err := dockerfiles.Asset("scripts/dockerfiles/Dockerfile-role")
	if err != nil {
		return nil, err
	}

	dockerfileTemplate := template.New("Dockerfile-role")
	context := map[string]interface{}{
		"base_image":    baseImage,
		"image_version": r.version,
		"role":          role,
		"license":       role.Jobs[0].Release.License.Filename,
		"notice":        role.Jobs[0].Release.Notice.Filename,
	}

	dockerfileTemplate, err = dockerfileTemplate.Parse(string(asset))
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	err = dockerfileTemplate.Execute(&output, context)
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

// GetRoleImageName generates a docker image name to be used as a role image
func GetRoleImageName(repository string, role *model.Role, version string) (string, error) {
	return util.SanitizeDockerName(fmt.Sprintf("%s-%s:%s-%s",
		repository,
		role.Name,
		role.Jobs[0].Release.Version,
		version,
	))
}

// GetRoleDevImageName generates a docker image name to be used as a dev role image
func GetRoleDevImageName(repository string, role *model.Role, version string) (string, error) {
	return util.SanitizeDockerName(fmt.Sprintf("%s-%s:%s",
		repository,
		role.Name,
		version,
	))
}
