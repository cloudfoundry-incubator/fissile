package builder

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"code.cloudfoundry.org/fissile/docker"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/scripts/dockerfiles"
	"code.cloudfoundry.org/fissile/util"
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
	BaseImageName      string
	DarkOpinionsPath   string
	DockerOrganization string
	DockerRegistry     string
	FissileVersion     string
	Force              bool
	Grapher            util.ModelGrapher
	LightOpinionsPath  string
	ManifestPath       string
	MetricsPath        string
	NoBuild            bool
	OutputDirectory    string
	RepositoryPrefix   string
	TagExtra           string
	UI                 *termui.UI
	Verbose            bool
	WorkerCount        int
}

// NewDockerPopulator returns a function which can populate a tar stream with the docker context to build the packages layer image with
func (r *RoleImageBuilder) NewDockerPopulator(instanceGroup *model.InstanceGroup) func(*tar.Writer) error {
	return func(tarWriter *tar.Writer) error {
		if len(instanceGroup.JobReferences) == 0 {
			return fmt.Errorf("Error - instance group %s has 0 jobs", instanceGroup.Name)
		}

		// Write out release license files
		releaseLicensesWritten := map[string]struct{}{}
		for _, jobReference := range instanceGroup.JobReferences {
			if _, ok := releaseLicensesWritten[jobReference.Release.Name]; !ok {
				if len(jobReference.Release.License.Files) == 0 {
					continue
				}

				releaseDir := filepath.Join("root/opt/fissile/share/doc", jobReference.Release.Name)

				for filename, contents := range jobReference.Release.License.Files {
					err := util.WriteToTarStream(tarWriter, contents, tar.Header{
						Name: filepath.Join(releaseDir, filename),
					})
					if err != nil {
						return fmt.Errorf("failed to write out release license file %s: %v", filename, err)
					}
				}
				releaseLicensesWritten[jobReference.Release.Name] = struct{}{}
			}
		}

		// Symlink compiled packages
		packageSet := map[string]string{}
		for _, jobReference := range instanceGroup.JobReferences {
			for _, pkg := range jobReference.Packages {
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
						r.UI.Printf("WARNING: duplicate package %s. Using package with fingerprint %s.\n",
							color.CyanString(pkg.Name), color.RedString(packageSet[pkg.Name]))
					}
				}
			}
		}

		// Copy jobs templates, spec configs and monit
		for _, jobReference := range instanceGroup.JobReferences {
			err := addJobTemplates(jobReference.Job, "root/var/vcap/jobs-src", tarWriter)
			if err != nil {
				return err
			}

			// Write spec into <ROOT_DIR>/var/vcap/job-src/<JOB>/config_spec.json
			configJSON, err := jobReference.WriteConfigs(instanceGroup, r.LightOpinionsPath, r.DarkOpinionsPath)
			if err != nil {
				return err
			}
			util.WriteToTarStream(tarWriter, configJSON, tar.Header{
				Name: filepath.Join("root/var/vcap/jobs-src", jobReference.Name, jobConfigSpecFilename),
			})
		}

		// Copy role startup scripts
		for script, sourceScriptPath := range instanceGroup.GetScriptPaths() {
			err := util.CopyFileToTarStream(tarWriter, sourceScriptPath, &tar.Header{
				Name: filepath.Join("root/opt/fissile/startup", script),
			})
			if err != nil {
				return fmt.Errorf("Error writing script %s: %s", script, err)
			}
		}

		// Copy manifest
		err := util.CopyFileToTarStream(tarWriter, r.ManifestPath, &tar.Header{
			Name: "root/opt/fissile/manifest.yaml",
		})
		if err != nil {
			return fmt.Errorf("Error writing manifest.yaml: %s", err)
		}

		// Generate run script
		runScriptContents, err := r.generateRunScript(instanceGroup, "run.sh")
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, runScriptContents, tar.Header{
			Name: "root/opt/fissile/run.sh",
			Mode: 0755,
		})
		if err != nil {
			return err
		}

		preStopScriptContents, err := r.generateRunScript(instanceGroup, "pre-stop.sh")
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, preStopScriptContents, tar.Header{
			Name: "root/opt/fissile/pre-stop.sh",
			Mode: 0755,
		})
		if err != nil {
			return err
		}

		jobsConfigContents, err := r.generateJobsConfig(instanceGroup)
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, jobsConfigContents, tar.Header{
			Name: "root/opt/fissile/job_config.json",
		})
		if err != nil {
			return err
		}

		// Copy readiness probe script
		readinessProbeScriptContents, err := r.generateRunScript(instanceGroup, "readiness-probe.sh")
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, readinessProbeScriptContents, tar.Header{
			Name: "root/opt/fissile/readiness-probe.sh",
			Mode: 0755,
		})
		if err != nil {
			return err
		}

		// Create env2conf templates file in /opt/fissile/env2conf.yml
		configTemplatesBytes, err := yaml.Marshal(instanceGroup.Configuration.Templates)
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, configTemplatesBytes, tar.Header{
			Name: "root/opt/fissile/env2conf.yml",
		})
		if err != nil {
			return err
		}

		// Generate Dockerfile
		buf := &bytes.Buffer{}
		if err := r.generateDockerfile(instanceGroup, buf); err != nil {
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

func (r *RoleImageBuilder) generateRunScript(instanceGroup *model.InstanceGroup, assetName string) ([]byte, error) {
	asset, err := dockerfiles.Asset(assetName)
	if err != nil {
		return nil, err
	}

	runScriptTemplate := template.New("role-script-" + assetName)
	runScriptTemplate.Funcs(template.FuncMap{
		"script_path": func(path string) string {
			if filepath.IsAbs(path) {
				return path
			}
			return filepath.Join("/opt/fissile/startup/", path)
		},
	})
	context := map[string]interface{}{
		"instance_group": instanceGroup,
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

func (r *RoleImageBuilder) generateJobsConfig(instanceGroup *model.InstanceGroup) ([]byte, error) {
	jobsConfig := make(map[string]map[string]interface{})

	for index, jobReference := range instanceGroup.JobReferences {
		jobsConfig[jobReference.Name] = make(map[string]interface{})
		jobsConfig[jobReference.Name]["base"] = fmt.Sprintf("/var/vcap/jobs-src/%s/config_spec.json", jobReference.Name)

		files := make(map[string]string)

		for _, file := range jobReference.Templates {
			src := fmt.Sprintf("/var/vcap/jobs-src/%s/templates/%s",
				jobReference.Name, file.SourcePath)
			dest := fmt.Sprintf("/var/vcap/jobs/%s/%s",
				jobReference.Name, file.DestinationPath)
			files[src] = dest
		}

		if instanceGroup.Type != "bosh-task" {
			src := fmt.Sprintf("/var/vcap/jobs-src/%s/monit", jobReference.Name)
			dest := fmt.Sprintf("/var/vcap/monit/%s.monitrc", jobReference.Name)
			files[src] = dest

			if index == 0 {
				files["/opt/fissile/monitrc.erb"] = "/etc/monitrc"
			}
		}

		jobsConfig[jobReference.Name]["files"] = files
	}

	jsonOut, err := json.Marshal(jobsConfig)
	if err != nil {
		return jsonOut, err
	}

	return jsonOut, nil
}

// generateDockerfile builds a docker file for a given role.
func (r *RoleImageBuilder) generateDockerfile(instanceGroup *model.InstanceGroup, outputFile io.Writer) error {
	asset, err := dockerfiles.Asset("Dockerfile-role")
	if err != nil {
		return err
	}

	dockerfileTemplate := template.New("Dockerfile-role")

	context := map[string]interface{}{
		"base_image":     r.BaseImageName,
		"instance_group": instanceGroup,
		"licenses":       instanceGroup.JobReferences[0].Release.License.Files,
	}

	dockerfileTemplate, err = dockerfileTemplate.Parse(string(asset))
	if err != nil {
		return err
	}

	return dockerfileTemplate.Execute(outputFile, context)
}

type roleBuildJob struct {
	instanceGroup *model.InstanceGroup
	builder       *RoleImageBuilder
	dockerManager dockerImageBuilder
	resultsCh     chan<- error
	abort         <-chan struct{}
}

func (j roleBuildJob) Run() {
	select {
	case <-j.abort:
		j.resultsCh <- nil
		return
	default:
	}

	j.resultsCh <- func() error {
		opinions, err := model.NewOpinions(j.builder.LightOpinionsPath, j.builder.DarkOpinionsPath)
		if err != nil {
			return err
		}

		devVersion, err := j.instanceGroup.GetRoleDevVersion(opinions, j.builder.TagExtra, j.builder.FissileVersion, j.builder.Grapher)
		if err != nil {
			return err
		}

		if j.builder.Grapher != nil {
			_ = j.builder.Grapher.GraphEdge(j.builder.BaseImageName, devVersion, nil)
		}

		var roleImageName string
		var outputPath string

		if j.builder.OutputDirectory == "" {
			roleImageName = GetRoleDevImageName(j.builder.DockerRegistry, j.builder.DockerOrganization, j.builder.RepositoryPrefix, j.instanceGroup, devVersion)
			outputPath = fmt.Sprintf("%s.tar", roleImageName)
		} else {
			roleImageName = GetRoleDevImageName("", "", j.builder.RepositoryPrefix, j.instanceGroup, devVersion)
			outputPath = filepath.Join(j.builder.OutputDirectory, fmt.Sprintf("%s.tar", roleImageName))
		}

		if !j.builder.Force {
			if j.builder.OutputDirectory == "" {
				if hasImage, err := j.dockerManager.HasImage(roleImageName); err != nil {
					return err
				} else if hasImage {
					j.builder.UI.Printf("Skipping build of role image %s because it exists\n", color.YellowString(j.instanceGroup.Name))
					return nil
				}
			} else {
				info, err := os.Stat(outputPath)
				if err == nil {
					if info.IsDir() {
						return fmt.Errorf("Output path %s exists but is a directory", outputPath)
					}
					j.builder.UI.Printf("Skipping build of role tarball %s because it exists\n", color.YellowString(outputPath))
					return nil
				}
				if !os.IsNotExist(err) {
					return err
				}
			}
		}

		if j.builder.MetricsPath != "" {
			seriesName := fmt.Sprintf("create-images::%s", roleImageName)

			stampy.Stamp(j.builder.MetricsPath, "fissile", seriesName, "start")
			defer stampy.Stamp(j.builder.MetricsPath, "fissile", seriesName, "done")
		}

		j.builder.UI.Printf("Creating Dockerfile for role %s ...\n", color.YellowString(j.instanceGroup.Name))
		dockerPopulator := j.builder.NewDockerPopulator(j.instanceGroup)

		if j.builder.NoBuild {
			j.builder.UI.Printf("Skipping build of role image %s because of flag\n", color.YellowString(j.instanceGroup.Name))
			return nil
		}

		if j.builder.OutputDirectory == "" {
			j.builder.UI.Printf("Building docker image of %s...\n", color.YellowString(j.instanceGroup.Name))

			log := new(bytes.Buffer)
			stdoutWriter := docker.NewFormattingWriter(
				log,
				docker.ColoredBuildStringFunc(roleImageName),
			)

			err := j.dockerManager.BuildImageFromCallback(roleImageName, stdoutWriter, dockerPopulator)
			if err != nil {
				log.WriteTo(j.builder.UI)
				return fmt.Errorf("Error building image: %s", err.Error())
			}
		} else {
			j.builder.UI.Printf("Building tarball of %s...\n", color.YellowString(j.instanceGroup.Name))

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

// Build triggers the building of the role docker images in parallel
func (r *RoleImageBuilder) Build(instanceGroups model.InstanceGroups) error {
	if r.WorkerCount < 1 {
		return fmt.Errorf("Invalid worker count %d", r.WorkerCount)
	}

	dockerManager, err := newDockerImageBuilder()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	if r.OutputDirectory != "" {
		if err = os.MkdirAll(r.OutputDirectory, 0755); err != nil {
			return fmt.Errorf("Error creating output directory: %s", err)
		}
	}

	workerLib.MaxJobs = r.WorkerCount
	worker := workerLib.NewWorker()

	resultsCh := make(chan error)
	abort := make(chan struct{})
	for _, instanceGroup := range instanceGroups {
		worker.Add(roleBuildJob{
			instanceGroup: instanceGroup,
			builder:       r,
			dockerManager: dockerManager,
			resultsCh:     resultsCh,
			abort:         abort,
		})
	}

	go worker.RunUntilDone()

	aborted := false
	for i := 0; i < len(instanceGroups); i++ {
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
func GetRoleDevImageName(registry, organization, repositoryPrefix string, instanceGroup *model.InstanceGroup, version string) string {
	var imageName string
	if registry != "" {
		imageName = registry + "/"
	}

	if organization != "" {
		imageName += util.SanitizeDockerName(organization) + "/"
	}

	imageName += util.SanitizeDockerName(fmt.Sprintf("%s-%s", repositoryPrefix, instanceGroup.Name))

	return fmt.Sprintf("%s:%s", imageName, util.SanitizeDockerName(version))
}
