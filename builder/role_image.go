package builder

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/dockerfiles"
	"github.com/hpcloud/fissile/util"

	"github.com/fatih/color"
	"github.com/hpcloud/termui"
	workerLib "github.com/jimmysawczuk/worker"
	"github.com/termie/go-shutil"
)

const (
	binPrefix = "bin"
)

var (
	// newDockerImageBuilder is a stub to be replaced by the unit test
	newDockerImageBuilder = func() (dockerImageBuilder, error) { return docker.NewImageManager() }
)

// dockerImageBuilder is the interface to shim around docker.RoleImageBuilder for the unit test
type dockerImageBuilder interface {
	BuildImage(dockerfileDirPath, name string, stdoutProcessor docker.ProcessOutStream) error
}

// RoleImageBuilder represents a builder of docker role images
type RoleImageBuilder struct {
	repository               string
	compiledPackagesPath     string
	targetPath               string
	defaultConsulAddress     string
	defaultConfigStorePrefix string
	version                  string
	fissileVersion           string
	ui                       *termui.UI
}

// NewRoleImageBuilder creates a new RoleImageBuilder
func NewRoleImageBuilder(repository, compiledPackagesPath, targetPath, defaultConsulAddress, defaultConfigStorePrefix, version, fissileVersion string, ui *termui.UI) *RoleImageBuilder {
	return &RoleImageBuilder{
		repository:               repository,
		compiledPackagesPath:     compiledPackagesPath,
		targetPath:               targetPath,
		defaultConsulAddress:     defaultConsulAddress,
		defaultConfigStorePrefix: defaultConfigStorePrefix,
		version:                  version,
		fissileVersion:           fissileVersion,
		ui:                       ui,
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
	for filename, contents := range role.Jobs[0].Release.License.Files {
		err := ioutil.WriteFile(filepath.Join(roleDir, filename), contents, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write out license file %s: %v", filename, err)
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
					r.ui.Printf("WARNING: duplicate package %s. Using package with fingerprint %s.\n",
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
				err := shutil.CopyTree(
					compiledDir,
					packageDir,
					&shutil.CopyTreeOptions{
						Symlinks:               true,
						Ignore:                 nil,
						CopyFunction:           shutil.Copy,
						IgnoreDanglingSymlinks: false},
				)

				if err != nil {
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
		"licenses":      role.Jobs[0].Release.License.Files,
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

type roleBuildJob struct {
	role          *model.Role
	builder       *RoleImageBuilder
	ui            *termui.UI
	noBuild       bool
	dockerManager dockerImageBuilder
	resultsCh     chan<- error
	abort         <-chan struct{}
	repository    string
	version       string
}

func (j roleBuildJob) Run() {
	select {
	case <-j.abort:
		return
	default:
	}

	j.ui.Printf("Creating Dockerfile for role %s ...\n", color.YellowString(j.role.Name))
	dockerfileDir, err := j.builder.CreateDockerfileDir(j.role)
	if err != nil {
		j.resultsCh <- fmt.Errorf("Error creating Dockerfile and/or assets for role %s: %s", j.role.Name, err.Error())
		return
	}

	if j.noBuild {
		j.ui.Println("Skipping image build because of flag.")
		j.resultsCh <- nil
		return
	}

	if !strings.HasSuffix(dockerfileDir, string(os.PathSeparator)) {
		dockerfileDir = fmt.Sprintf("%s%c", dockerfileDir, os.PathSeparator)
	}

	j.ui.Printf("Building docker image in %s ...\n", color.YellowString(dockerfileDir))

	roleImageName := GetRoleImageName(j.repository, j.role, j.version)

	err = j.dockerManager.BuildImage(dockerfileDir, roleImageName, newColoredLogger(roleImageName, j.ui))
	if err != nil {
		j.resultsCh <- fmt.Errorf("Error building image: %s", err.Error())
		return
	}
	j.resultsCh <- nil
}

// BuildRoleImages triggers the building of the role docker images in parallel
func (r *RoleImageBuilder) BuildRoleImages(roles []*model.Role, repository, version string, noBuild bool, workerCount int) error {
	if workerCount < 1 {
		return fmt.Errorf("Invalid worker count %d", workerCount)
	}

	dockerManager, err := newDockerImageBuilder()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	workerLib.MaxJobs = workerCount
	worker := workerLib.NewWorker()

	resultsCh := make(chan error)
	abort := make(chan struct{})
	for _, role := range roles {
		worker.Add(roleBuildJob{
			role:          role,
			builder:       r,
			ui:            r.ui,
			noBuild:       noBuild,
			dockerManager: dockerManager,
			resultsCh:     resultsCh,
			abort:         abort,
			repository:    repository,
			version:       version,
		})
	}
	go func() {
		aborted := false
		for i := 0; i < len(roles); i++ {
			if result := <-resultsCh; result != nil {
				if !aborted {
					close(abort)
					aborted = true
				}
				err = result
			}
		}
	}()
	worker.RunUntilDone()

	return err
}

// GetRoleImageName generates a docker image name to be used as a role image
func GetRoleImageName(repository string, role *model.Role, version string) string {
	return util.SanitizeDockerName(fmt.Sprintf("%s-%s:%s-%s",
		repository,
		role.Name,
		role.Jobs[0].Release.Version,
		version,
	))
}

// GetRoleDevImageName generates a docker image name to be used as a dev role image
func GetRoleDevImageName(repository string, role *model.Role, version string) string {
	return util.SanitizeDockerName(fmt.Sprintf("%s-%s:%s",
		repository,
		role.Name,
		version,
	))
}

func newColoredLogger(roleImageName string, ui *termui.UI) func(io.Reader) {
	return func(stdout io.Reader) {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			ui.Println(color.GreenString("build-%s > %s", color.MagentaString(roleImageName), color.WhiteString(scanner.Text())))
		}
	}
}
