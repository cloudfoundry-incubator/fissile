package builder

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/SUSE/fissile/docker"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/scripts/dockerfiles"
	"github.com/SUSE/fissile/util"
	"github.com/SUSE/stampy"
	"github.com/SUSE/termui"

	"github.com/fatih/color"
	workerLib "github.com/jimmysawczuk/worker"
	"gopkg.in/yaml.v2"
)

const (
	binPrefix             = "bin"
	jobConfigSpecFilename = "config_spec.json"
)

var (
	// newDockerImageBuilder is a stub to be replaced by the unit test
	newDockerImageBuilder = func() (dockerImageBuilder, error) { return docker.NewImageManager() }
)

// dockerImageBuilder is the interface to shim around docker.RoleImageBuilder for the unit test
type dockerImageBuilder interface {
	HasImage(imageName string) (bool, error)
	BuildImage(dockerfileDirPath, name string, stdoutProcessor io.WriteCloser) error
	BuildImageFromCallback(name string, stdoutWriter io.Writer, callback func(*tar.Writer) error) error
}

// RoleImageBuilder represents a builder of docker role images
type RoleImageBuilder struct {
	repository           string
	compiledPackagesPath string
	targetPath           string
	metricsPath          string
	fissileVersion       string
	lightOpinionsPath    string
	darkOpinionsPath     string
	ui                   *termui.UI
}

// NewRoleImageBuilder creates a new RoleImageBuilder
func NewRoleImageBuilder(repository, compiledPackagesPath, targetPath, lightOpinionsPath, darkOpinionsPath, metricsPath, fissileVersion string, ui *termui.UI) (*RoleImageBuilder, error) {
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, err
	}

	return &RoleImageBuilder{
		repository:           repository,
		compiledPackagesPath: compiledPackagesPath,
		targetPath:           targetPath,
		metricsPath:          metricsPath,
		fissileVersion:       fissileVersion,
		lightOpinionsPath:    lightOpinionsPath,
		darkOpinionsPath:     darkOpinionsPath,
		ui:                   ui,
	}, nil
}

// NewDockerPopulator returns a function which can populate a tar stream with the docker context to build the packages layer image with
func (r *RoleImageBuilder) NewDockerPopulator(role *model.Role, baseImageName string) func(*tar.Writer) error {
	return func(tarWriter *tar.Writer) error {
		if len(role.Jobs) == 0 {
			return fmt.Errorf("Error - role %s has 0 jobs", role.Name)
		}

		// Write out release license files
		releaseLicensesWritten := map[string]struct{}{}
		for _, job := range role.Jobs {
			if _, ok := releaseLicensesWritten[job.Release.Name]; !ok {
				if len(job.Release.License.Files) == 0 {
					continue
				}

				releaseDir := filepath.Join("root/opt/scf/share/doc", job.Release.Name)

				for filename, contents := range job.Release.License.Files {
					err := util.WriteToTarStream(tarWriter, contents, tar.Header{
						Name: filepath.Join(releaseDir, filename),
					})
					if err != nil {
						return fmt.Errorf("failed to write out release license file %s: %v", filename, err)
					}
				}
				releaseLicensesWritten[job.Release.Name] = struct{}{}
			}
		}

		// Symlink compiled packages
		packageSet := map[string]string{}
		for _, job := range role.Jobs {
			for _, pkg := range job.Packages {
				if _, ok := packageSet[pkg.Name]; !ok {
					err := util.WriteToTarStream(tarWriter, nil, tar.Header{
						Name:     filepath.Join("root/var/vcap/packages", pkg.Name),
						Typeflag: tar.TypeSymlink,
						Linkname: filepath.Join("..", "packages-src", pkg.Fingerprint),
					})
					if err != nil {
						return fmt.Errorf("failed to write package symlink for %s: %s", pkg.Name, err)
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
		for _, job := range role.Jobs {
			templates := make(map[string]*model.JobTemplate)
			for _, template := range job.Templates {
				sourcePath := filepath.Clean(filepath.Join("templates", template.SourcePath))
				templates[filepath.ToSlash(sourcePath)] = template
			}

			sourceTgz, err := os.Open(job.Path)
			if err != nil {
				return fmt.Errorf("Error reading archive for job %s (%s): %s", job.Name, job.Path, err)
			}
			defer sourceTgz.Close()
			err = util.TargzIterate(job.Path, sourceTgz, func(reader *tar.Reader, header *tar.Header) error {
				filePath := filepath.ToSlash(filepath.Clean(header.Name))
				if filePath == "job.MF" {
					return nil
				}
				header.Name = filepath.Join("root/var/vcap/jobs-src", job.Name, header.Name)
				if template, ok := templates[filePath]; ok {
					if strings.HasPrefix(template.DestinationPath, fmt.Sprintf("%s%c", binPrefix, os.PathSeparator)) {
						header.Mode = 0755
					} else {
						header.Mode = 0644
					}
				}
				if err = tarWriter.WriteHeader(header); err != nil {
					return fmt.Errorf("Error writing header %s for job %s: %s", filePath, job.Name, err)
				}
				if _, err = io.Copy(tarWriter, reader); err != nil {
					return fmt.Errorf("Error writing %s for job %s: %s", filePath, job.Name, err)
				}
				return nil
			})

			// Write spec into <ROOT_DIR>/var/vcap/job-src/<JOB>/config_spec.json
			configJSON, err := job.WriteConfigs(role, r.lightOpinionsPath, r.darkOpinionsPath)
			if err != nil {
				return err
			}
			util.WriteToTarStream(tarWriter, configJSON, tar.Header{
				Name: filepath.Join("root/var/vcap/jobs-src", job.Name, jobConfigSpecFilename),
			})
		}

		// Copy role startup scripts
		for script, sourceScriptPath := range role.GetScriptPaths() {
			err := util.CopyFileToTarStream(tarWriter, sourceScriptPath, &tar.Header{
				Name: filepath.Join("root/opt/scf/startup", script),
			})
			if err != nil {
				return fmt.Errorf("Error writing script %s: %s", script, err)
			}
		}

		// Generate run script
		runScriptContents, err := r.generateRunScript(role)
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, runScriptContents, tar.Header{
			Name: "root/opt/scf/run.sh",
		})
		if err != nil {
			return err
		}

		jobsConfigContents, err := r.generateJobsConfig(role)
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, jobsConfigContents, tar.Header{
			Name: "root/opt/scf/job_config.json",
		})
		if err != nil {
			return err
		}

		// Create env2conf templates file in /opt/scf/env2conf.yml
		configTemplatesBytes, err := yaml.Marshal(role.Configuration.Templates)
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, configTemplatesBytes, tar.Header{
			Name: "root/opt/scf/env2conf.yml",
		})
		if err != nil {
			return err
		}

		// Generate Dockerfile
		buf := &bytes.Buffer{}
		if err := r.generateDockerfile(role, baseImageName, buf); err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, buf.Bytes(), tar.Header{
			Name: "Dockerfile",
		})
		if err != nil {
			return err
		}

		return nil
	}
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

func (r *RoleImageBuilder) generateJobsConfig(role *model.Role) ([]byte, error) {
	jobsConfig := make(map[string]map[string]interface{})

	for index, job := range role.Jobs {
		jobsConfig[job.Name] = make(map[string]interface{})
		jobsConfig[job.Name]["base"] = fmt.Sprintf("/var/vcap/jobs-src/%s/config_spec.json", job.Name)

		files := make(map[string]string)

		for _, file := range job.Templates {
			src := fmt.Sprintf("/var/vcap/jobs-src/%s/templates/%s",
				job.Name, file.SourcePath)
			dest := fmt.Sprintf("/var/vcap/jobs/%s/%s",
				job.Name, file.DestinationPath)
			files[src] = dest
		}

		if role.Type != "bosh-task" {
			src := fmt.Sprintf("/var/vcap/jobs-src/%s/monit", job.Name)
			dest := fmt.Sprintf("/var/vcap/monit/%s.monitrc", job.Name)
			files[src] = dest

			if index == 0 {
				files["/opt/scf/monitrc.erb"] = "/etc/monitrc"
			}
		}

		jobsConfig[job.Name]["files"] = files
	}

	jsonOut, err := json.Marshal(jobsConfig)
	if err != nil {
		return jsonOut, err
	}

	return jsonOut, nil
}

// generateDockerfile builds a docker file for a given role.
func (r *RoleImageBuilder) generateDockerfile(role *model.Role, baseImageName string, outputFile io.Writer) error {
	asset, err := dockerfiles.Asset("Dockerfile-role")
	if err != nil {
		return err
	}

	dockerfileTemplate := template.New("Dockerfile-role")

	context := map[string]interface{}{
		"base_image": baseImageName,
		"role":       role,
		"licenses":   role.Jobs[0].Release.License.Files,
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
	role            *model.Role
	builder         *RoleImageBuilder
	ui              *termui.UI
	force           bool
	noBuild         bool
	dockerManager   dockerImageBuilder
	outputDirectory string
	resultsCh       chan<- error
	abort           <-chan struct{}
	registry        string
	organization    string
	repository      string
	baseImageName   string
	verbosity       util.Verbosity
}

func (j roleBuildJob) Run() {
	select {
	case <-j.abort:
		j.resultsCh <- nil
		return
	default:
	}

	j.resultsCh <- func() error {
		opinions, err := model.NewOpinions(j.builder.lightOpinionsPath, j.builder.darkOpinionsPath)
		if err != nil {
			return err
		}

		devVersion, err := j.role.GetRoleDevVersion(opinions, j.builder.fissileVersion, j.verbosity)
		if err != nil {
			return err
		}

		var roleImageName string
		var outputPath string

		if j.outputDirectory == "" {
			roleImageName = GetRoleDevImageName(j.registry, j.organization, j.repository, j.role, devVersion)
			outputPath = fmt.Sprintf("%s.tar", roleImageName)
		} else {
			roleImageName = GetRoleDevImageName("", "", j.repository, j.role, devVersion)
			outputPath = filepath.Join(j.outputDirectory, fmt.Sprintf("%s.tar", roleImageName))
		}

		if !j.force {
			if j.outputDirectory == "" {
				if hasImage, err := j.dockerManager.HasImage(roleImageName); err != nil {
					return err
				} else if hasImage {
					j.ui.Printf("Skipping build of role image %s because it exists\n", color.YellowString(j.role.Name))
					return nil
				}
			} else {
				info, err := os.Stat(outputPath)
				if err == nil {
					if info.IsDir() {
						return fmt.Errorf("Output path %s exists but is a directory", outputPath)
					}
					j.ui.Printf("Skipping build of role tarball %s because it exists\n", color.YellowString(outputPath))
					return nil
				}
				if !os.IsNotExist(err) {
					return err
				}
			}
		}

		if j.builder.metricsPath != "" {
			seriesName := fmt.Sprintf("create-role-images::%s", roleImageName)

			stampy.Stamp(j.builder.metricsPath, "fissile", seriesName, "start")
			defer stampy.Stamp(j.builder.metricsPath, "fissile", seriesName, "done")
		}

		j.ui.Printf("Creating Dockerfile for role %s ...\n", color.YellowString(j.role.Name))
		dockerPopulator := j.builder.NewDockerPopulator(j.role, j.baseImageName)

		if j.noBuild {
			j.ui.Printf("Skipping build of role image %s because of flag\n", color.YellowString(j.role.Name))
			return nil
		}

		if j.outputDirectory == "" {
			j.ui.Printf("Building docker image of %s...\n", color.YellowString(j.role.Name))

			log := new(bytes.Buffer)
			stdoutWriter := docker.NewFormattingWriter(
				log,
				docker.ColoredBuildStringFunc(roleImageName),
			)

			err := j.dockerManager.BuildImageFromCallback(roleImageName, stdoutWriter, dockerPopulator)
			if err != nil {
				log.WriteTo(j.ui)
				return fmt.Errorf("Error building image: %s", err.Error())
			}
		} else {
			j.ui.Printf("Building tarball of %s...\n", color.YellowString(j.role.Name))

			tarFile, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("Failed to create tar file %s: %s", outputPath, err)
			}
			tarWriter := tar.NewWriter(tarFile)

			err = dockerPopulator(tarWriter)
			if err != nil {
				return fmt.Errorf("Failed to populate tar file %s: %s", outputPath, err)
			}

			err = tarWriter.Close()
			if err != nil {
				return fmt.Errorf("Failed to close tar file %s: %s", outputPath, err)
			}
		}
		return nil
	}()
}

// BuildRoleImages triggers the building of the role docker images in parallel
func (r *RoleImageBuilder) BuildRoleImages(roles model.Roles, registry, organization, repository, baseImageName, outputDirectory string, force, noBuild bool, workerCount int, verbosity util.Verbosity) error {
	if workerCount < 1 {
		return fmt.Errorf("Invalid worker count %d", workerCount)
	}

	dockerManager, err := newDockerImageBuilder()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	if outputDirectory != "" {
		if err = os.MkdirAll(outputDirectory, 0755); err != nil {
			return fmt.Errorf("Error creating output directory: %s", err)
		}
	}

	workerLib.MaxJobs = workerCount
	worker := workerLib.NewWorker()

	resultsCh := make(chan error)
	abort := make(chan struct{})
	for _, role := range roles {
		worker.Add(roleBuildJob{
			role:            role,
			builder:         r,
			ui:              r.ui,
			force:           force,
			noBuild:         noBuild,
			dockerManager:   dockerManager,
			outputDirectory: outputDirectory,
			resultsCh:       resultsCh,
			abort:           abort,
			registry:        registry,
			organization:    organization,
			repository:      repository,
			baseImageName:   baseImageName,
			verbosity:       verbosity,
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
func GetRoleDevImageName(registry, organization, repository string, role *model.Role, version string) string {
	var imageName string
	if registry != "" {
		imageName = registry + "/"
	}

	if organization != "" {
		imageName += util.SanitizeDockerName(organization) + "/"
	}

	imageName += util.SanitizeDockerName(fmt.Sprintf("%s-%s", repository, role.Name))

	return fmt.Sprintf("%s:%s", imageName, util.SanitizeDockerName(version))
}
