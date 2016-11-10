package builder

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hpcloud/fissile/config-store"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/dockerfiles"
	"github.com/hpcloud/fissile/util"

	"github.com/fatih/color"
	"github.com/hpcloud/termui"
	workerLib "github.com/jimmysawczuk/worker"
	"github.com/termie/go-shutil"
	"gopkg.in/yaml.v2"
)

const (
	binPrefix             = "bin"
	jobConfigSpecFilename = "config_spec"
)

var (
	// newDockerImageBuilder is a stub to be replaced by the unit test
	newDockerImageBuilder = func() (dockerImageBuilder, error) { return docker.NewImageManager() }
)

// dockerImageBuilder is the interface to shim around docker.RoleImageBuilder for the unit test
type dockerImageBuilder interface {
	HasImage(imageName string) (bool, error)
	BuildImage(dockerfileDirPath, name string, stdoutProcessor io.WriteCloser) error
}

// RoleImageBuilder represents a builder of docker role images
type RoleImageBuilder struct {
	repository           string
	compiledPackagesPath string
	targetPath           string
	version              string
	fissileVersion       string
	ui                   *termui.UI
}

// NewRoleImageBuilder creates a new RoleImageBuilder
func NewRoleImageBuilder(repository, compiledPackagesPath, targetPath, version, fissileVersion string, ui *termui.UI) (*RoleImageBuilder, error) {
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, err
	}
	return &RoleImageBuilder{
		repository:           repository,
		compiledPackagesPath: compiledPackagesPath,
		targetPath:           targetPath,
		version:              version,
		fissileVersion:       fissileVersion,
		ui:                   ui,
	}, nil
}

// CreateDockerfileDir generates a Dockerfile and assets in the targetDir and returns a path to the dir
func (r *RoleImageBuilder) CreateDockerfileDir(role *model.Role, baseImageName string) (string, error) {
	if len(role.Jobs) == 0 {
		return "", fmt.Errorf("Error - role %s has 0 jobs", role.Name)
	}

	succeeded := false
	roleDir, err := ioutil.TempDir(r.targetPath, fmt.Sprintf("role-%s", role.Name))
	if err != nil {
		return "", err
	}
	defer func() {
		if !succeeded {
			os.RemoveAll(roleDir)
		}
	}()

	// Create a dir for the role
	rootDir := filepath.Join(roleDir, "root")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		return "", err
	}

	// Write out release license files
	releaseLicensesWritten := map[string]struct{}{}
	for _, job := range role.Jobs {
		docDir := filepath.Join(rootDir, "opt/hcf/share/doc")

		if _, ok := releaseLicensesWritten[job.Release.Name]; !ok {
			if len(job.Release.License.Files) == 0 {
				continue
			}

			releaseDir := filepath.Join(docDir, job.Release.Name)
			if err := os.MkdirAll(releaseDir, 0755); err != nil {
				return "", err
			}

			for filename, contents := range job.Release.License.Files {
				err := ioutil.WriteFile(filepath.Join(releaseDir, filename), contents, 0644)
				if err != nil {
					return "", fmt.Errorf("failed to write out release license file %s: %v", filename, err)
				}
			}
		}
	}

	// Symlink compiled packages
	packagesDir := filepath.Join(rootDir, "var/vcap/packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		return "", err
	}
	packageSet := map[string]string{}
	for _, job := range role.Jobs {
		for _, pkg := range job.Packages {
			if _, ok := packageSet[pkg.Name]; !ok {
				sourceDir := filepath.Join("..", "packages-src", pkg.Fingerprint)
				packageDir := filepath.Join(packagesDir, pkg.Name)
				err := os.Symlink(sourceDir, packageDir)
				if err != nil {
					return "", err
				}
				packageSet[pkg.Name] = pkg.Fingerprint
			} else {
				if pkg.Fingerprint != packageSet[pkg.Name] {
					r.ui.Printf("WARNING: duplicate package %s. Using package with fingerprint %s.\n",
						color.CyanString(pkg.Name), color.RedString(packageSet[pkg.Name]))
				}
			}
		}
	}

	// Copy jobs templates, spec configs and monit
	jobsDir := filepath.Join(rootDir, "var/vcap/jobs-src")
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		return "", err
	}
	jsonSpecsDir := filepath.Join(rootDir, "opt/hcf/specs")
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

		// Symlink spec configuration file
		// from <ROOT_DIR>/opt/hcf/specs/<ROLE_NAME>/<JOB>.json
		specConfigSource := filepath.Join(jsonSpecsDir, role.Name, job.Name+configstore.JobConfigFileExtension)
		// into <ROOT_DIR>/var/vcap/job-src/<JOB>/config_spec.json
		specConfigDestination := filepath.Join(jobDir, jobConfigSpecFilename+configstore.JobConfigFileExtension)
		// The symlink must be relative, since the path is different in the docker image
		specConfigRelativeSource, err := filepath.Rel(filepath.Dir(specConfigDestination), specConfigSource)
		if err != nil {
			return "", err
		}
		if err := os.Symlink(specConfigRelativeSource, specConfigDestination); err != nil {
			return "", err
		}
	}

	// Copy role startup scripts
	startupDir := filepath.Join(rootDir, "opt/hcf/startup")
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
	runScriptPath := filepath.Join(rootDir, "opt/hcf/run.sh")
	if err := ioutil.WriteFile(runScriptPath, runScriptContents, 0744); err != nil {
		return "", err
	}

	// Create env2conf templates file in /opt/hcf/env2conf.yml
	configTemplatesBytes, err := yaml.Marshal(role.Configuration.Templates)
	if err != nil {
		return "", err
	}
	configTemplatesFilePath := filepath.Join(rootDir, "opt/hcf/env2conf.yml")
	if err := ioutil.WriteFile(configTemplatesFilePath, configTemplatesBytes, 0644); err != nil {
		return "", err
	}

	// Generate Dockerfile
	dockerfile, err := os.Create(filepath.Join(roleDir, "Dockerfile"))
	if err != nil {
		return "", err
	}
	defer dockerfile.Close()
	if err := r.generateDockerfile(role, baseImageName, dockerfile); err != nil {
		return "", err
	}

	succeeded = true
	return roleDir, nil
}

func isPreStart(s string) bool {
	return strings.HasSuffix(s, "/bin/pre-start")
}

func (r *RoleImageBuilder) generateRunScript(role *model.Role) ([]byte, error) {
	asset, err := dockerfiles.Asset("run.sh")
	if err != nil {
		return nil, err
	}

	runScriptTemplate := template.New("role-runscript")
	runScriptTemplate.Funcs(template.FuncMap{
		"is_abs":       filepath.IsAbs,
		"is_pre_start": isPreStart,
	})
	context := map[string]interface{}{
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

// generateDockerfile builds a docker file for a given role.
func (r *RoleImageBuilder) generateDockerfile(role *model.Role, baseImageName string, outputFile io.Writer) error {
	asset, err := dockerfiles.Asset("Dockerfile-role")
	if err != nil {
		return err
	}

	dockerfileTemplate := template.New("Dockerfile-role")

	context := map[string]interface{}{
		"base_image":    baseImageName,
		"image_version": r.version,
		"role":          role,
		"licenses":      role.Jobs[0].Release.License.Files,
	}

	dockerfileTemplate, err = dockerfileTemplate.Parse(string(asset))
	if err != nil {
		return err
	}

	if err := dockerfileTemplate.Execute(outputFile, context); err != nil {
		return err
	}

	return nil
}

type roleBuildJob struct {
	role          *model.Role
	builder       *RoleImageBuilder
	ui            *termui.UI
	force         bool
	noBuild       bool
	dockerManager dockerImageBuilder
	resultsCh     chan<- error
	abort         <-chan struct{}
	repository    string
	baseImageName string
}

func (j roleBuildJob) Run() {
	select {
	case <-j.abort:
		j.resultsCh <- nil
		return
	default:
	}

	versionHash, err := j.role.GetRoleDevVersion()
	if err != nil {
		j.resultsCh <- err
		return
	}
	roleImageName := GetRoleDevImageName(j.repository, j.role, versionHash)
	if !j.force {
		if hasImage, err := j.dockerManager.HasImage(roleImageName); err != nil {
			j.resultsCh <- err
			return
		} else if hasImage {
			j.ui.Printf("Skipping build of role image %s because it exists\n", color.YellowString(j.role.Name))
			j.resultsCh <- nil
			return
		}
	}

	j.ui.Printf("Creating Dockerfile for role %s ...\n", color.YellowString(j.role.Name))
	dockerfileDir, err := j.builder.CreateDockerfileDir(j.role, j.baseImageName)
	if err != nil {
		j.resultsCh <- fmt.Errorf("Error creating Dockerfile and/or assets for role %s: %s", j.role.Name, err.Error())
		return
	}

	if j.noBuild {
		j.ui.Printf("Skipping build of role image %s because of flag\n", color.YellowString(j.role.Name))
		j.resultsCh <- nil
		return
	}

	if !strings.HasSuffix(dockerfileDir, string(os.PathSeparator)) {
		dockerfileDir = fmt.Sprintf("%s%c", dockerfileDir, os.PathSeparator)
	}

	j.ui.Printf("Building docker image of %s in %s ...\n", color.YellowString(j.role.Name), color.YellowString(dockerfileDir))

	log := new(bytes.Buffer)
	stdoutWriter := docker.NewFormattingWriter(
		log,
		docker.ColoredBuildStringFunc(roleImageName),
	)

	err = j.dockerManager.BuildImage(dockerfileDir, roleImageName, stdoutWriter)
	if err != nil {
		log.WriteTo(j.ui)
		j.resultsCh <- fmt.Errorf("Error building image: %s", err.Error())
		return
	}
	j.resultsCh <- nil
}

// BuildRoleImages triggers the building of the role docker images in parallel
func (r *RoleImageBuilder) BuildRoleImages(roles model.Roles, repository, baseImageName string, force, noBuild bool, workerCount int) error {
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
			force:         force,
			noBuild:       noBuild,
			dockerManager: dockerManager,
			resultsCh:     resultsCh,
			abort:         abort,
			repository:    repository,
			baseImageName: baseImageName,
		})
	}

	go worker.RunUntilDone()

	aborted := false
	for i := 0; i < len(roles); i++ {
		result := <-resultsCh
		if result != nil {
			if !aborted {
				close(abort)
				aborted = true
			}
			err = result
		}
	}
	return err
}

// GetRoleDevImageName generates a docker image name to be used as a dev role image
func GetRoleDevImageName(repository string, role *model.Role, version string) string {
	return util.SanitizeDockerName(fmt.Sprintf("%s-%s:%s",
		repository,
		role.Name,
		version,
	))
}
