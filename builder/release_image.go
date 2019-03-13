package builder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"code.cloudfoundry.org/fissile/compilator"
	"code.cloudfoundry.org/fissile/docker"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/scripts/compilation"
	"code.cloudfoundry.org/fissile/scripts/dockerfiles"
	"code.cloudfoundry.org/fissile/util"
	"github.com/SUSE/stampy"
	"github.com/SUSE/termui"
	"github.com/fatih/color"
	dockerclient "github.com/fsouza/go-dockerclient"
	workerLib "github.com/jimmysawczuk/worker"
	"github.com/pkg/errors"
)

// ReleasesImageBuilder represents a builder of docker release images.
type ReleasesImageBuilder struct {
	CompilationCacheConfig string
	CompilationDir         string
	DockerNetworkMode      string
	DockerOrganization     string
	DockerRegistry         string
	FissileVersion         string
	Force                  bool
	Grapher                util.ModelGrapher
	MetricsPath            string
	NoBuild                bool
	OutputDirectory        string
	RepositoryPrefix       string
	StemcellName           string
	StreamPackages         bool
	UI                     *termui.UI
	Verbose                bool
	WithoutDocker          bool
	WorkerCount            int
}

type releaseBuildJob struct {
	release       *model.Release
	builder       *ReleasesImageBuilder
	dockerManager dockerImageBuilder
	resultsCh     chan<- error
	abort         <-chan struct{}
}

func addJobTemplates(job *model.Job, path string, tarWriter *tar.Writer) error {
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
	return util.TargzIterate(job.Path, sourceTgz, func(reader *tar.Reader, header *tar.Header) error {
		filePath := filepath.ToSlash(filepath.Clean(header.Name))
		header.Name = filepath.Join(path, job.Name, header.Name)
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
}

// NewDockerPopulator returns a function which can populate a tar stream with the docker context to build the packages layer image with.
func (r *ReleasesImageBuilder) NewDockerPopulator(release *model.Release) func(*tar.Writer) error {
	return func(tarWriter *tar.Writer) error {
		// Generate dockerfile
		var dockerfile bytes.Buffer
		err := r.generateDockerfile(&dockerfile, release, map[string]string{})
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, dockerfile.Bytes(), tar.Header{
			Name: "Dockerfile",
		})
		if err != nil {
			return err
		}

		// Write release.MF to tar stream
		manifest, err := ioutil.ReadFile(filepath.Join(release.Path, "release.MF"))
		if err != nil {
			return err
		}
		err = util.WriteToTarStream(tarWriter, manifest, tar.Header{
			Name: "root/var/vcap/release.MF",
		})
		if err != nil {
			return err
		}

		for _, job := range release.Jobs {
			// Insert the compiled packages into the tar stream
			for _, pkg := range job.Packages {
				walker := &tarWalker{
					stream: tarWriter,
					root:   pkg.GetPackageCompiledDir(r.CompilationDir),
					prefix: filepath.Join("root/var/vcap/packages", pkg.Name),
				}
				if err = filepath.Walk(walker.root, walker.walk); err != nil {
					return err
				}
			}

			// Add jobs templates (including unwanted monit template)
			err := addJobTemplates(job, "root/var/vcap/jobs-src", tarWriter)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// generateDockerfile builds a docker file for the shared packages layer.
func (r *ReleasesImageBuilder) generateDockerfile(outputFile io.Writer, release *model.Release, labels map[string]string) error {
	labels["version.generator.fissile"] = strings.Replace(r.FissileVersion, "+", "_", -1)
	context := map[string]interface{}{
		"base_image": r.StemcellName,
		"labels":     labels,
		"name":       release.Name,
		"version":    release.Version,
	}
	asset, err := dockerfiles.Asset("Dockerfile-release")
	if err != nil {
		return err
	}

	dockerfileTemplate := template.New("Dockerfile")
	dockerfileTemplate, err = dockerfileTemplate.Parse(string(asset))
	if err != nil {
		return err
	}

	return dockerfileTemplate.Execute(outputFile, context)
}

func (j releaseBuildJob) imageName() (string, error) {
	imageManager, err := docker.NewImageManager()
	if err != nil {
		return "", errors.Wrap(err, "Connecting to docker daemon")
	}

	listOptions := dockerclient.ListImagesOptions{Filter: j.builder.StemcellName}
	matches, err := imageManager.ListImages(listOptions)
	if err != nil || len(matches) == 0 {
		return "", errors.Wrap(err, fmt.Sprintf("Retrieving stemcell image '%s'", j.builder.StemcellName))
	}
	stemcellImage := matches[0]
	stemcellFlavor := stemcellImage.Labels["stemcell-flavor"]
	stemcellVersion := stemcellImage.Labels["stemcell-version"]
	if stemcellFlavor == "" || stemcellVersion == "" {
		j.builder.UI.Printf("Warning: Stemcell '%s' does not include required labels 'stemcell-flavor' and 'stemcell-version'\n", j.builder.StemcellName)
		stemcellFlavor = "unknown"
		idRegexp := regexp.MustCompile(".*:([a-z0-9]{12})")
		stemcellVersion = idRegexp.FindStringSubmatch(stemcellImage.ID)[1]
	}

	var imageName string
	if j.builder.DockerRegistry != "" {
		imageName = j.builder.DockerRegistry + "/"
	}
	if j.builder.DockerOrganization != "" {
		imageName += util.SanitizeDockerName(j.builder.DockerOrganization) + "/"
	}
	imageName += util.SanitizeDockerName(util.PrefixString(j.release.Name, j.builder.RepositoryPrefix, "-"))

	fissileVersion := strings.Replace(j.builder.FissileVersion, "fissile-", "", -1)
	fissileVersion = strings.Replace(fissileVersion, "+", "_", -1)
	tag := fmt.Sprintf("%s-%s", stemcellFlavor, stemcellVersion)
	tag = tag + fmt.Sprintf("-%s-%s", fissileVersion, j.release.Version)

	return fmt.Sprintf("%s:%s", imageName, tag), nil
}

func (j releaseBuildJob) CompileRelease() error {
	r := j.builder

	packageStorage, err := compilator.NewPackageStorageFromConfig(r.CompilationCacheConfig, r.CompilationDir, r.StemcellName)
	if err != nil {
		return err
	}
	var comp *compilator.Compilator
	if r.WithoutDocker {
		comp, err = compilator.NewMountNSCompilator(
			r.CompilationDir,
			r.MetricsPath,
			r.StemcellName,
			compilation.LinuxBase,
			r.FissileVersion,
			r.UI,
			r.Grapher,
			packageStorage,
		)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %s", err.Error())
		}
	} else {
		dockerManager, err := docker.NewImageManager()
		if err != nil {
			return fmt.Errorf("Error connecting to docker: %s", err.Error())
		}

		comp, err = compilator.NewDockerCompilator(
			dockerManager,
			r.CompilationDir,
			r.MetricsPath,
			r.StemcellName,
			compilation.LinuxBase,
			r.FissileVersion,
			r.DockerNetworkMode,
			false,
			r.UI,
			r.Grapher,
			packageStorage,
			r.StreamPackages,
		)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %s", err.Error())
		}
	}

	err = comp.Compile(j.builder.WorkerCount, model.Releases{j.release}, nil, j.builder.Verbose)
	if err != nil {
		return fmt.Errorf("Error compiling packages: %s", err.Error())
	}

	return nil
}

func (j releaseBuildJob) Run() {
	r := j.builder

	select {
	case <-j.abort:
		j.resultsCh <- nil
		return
	default:
	}

	j.resultsCh <- func() error {
		if r.Grapher != nil {
			_ = r.Grapher.GraphEdge(r.StemcellName, j.release.Version, nil)
		}

		imageName, err := j.imageName()
		if err != nil {
			return err
		}
		outputPath := fmt.Sprintf("%s.tar", imageName)

		if r.OutputDirectory != "" {
			outputPath = filepath.Join(r.OutputDirectory, outputPath)
		}

		if !r.Force {
			if r.OutputDirectory == "" {
				if hasImage, err := j.dockerManager.HasImage(imageName); err != nil {
					return err
				} else if hasImage {
					r.UI.Printf("Skipping build of release image %s because it exists\n", color.YellowString(j.release.Name))
					return nil
				}
			} else {
				info, err := os.Stat(outputPath)
				if err == nil {
					if info.IsDir() {
						return fmt.Errorf("Output path %s exists but is a directory", outputPath)
					}
					r.UI.Printf("Skipping build of release tarball %s because it exists\n", color.YellowString(outputPath))
					return nil
				}
				if !os.IsNotExist(err) {
					return err
				}
			}
		}

		err = j.CompileRelease()
		if err != nil {
			return err
		}

		if r.MetricsPath != "" {
			seriesName := fmt.Sprintf("create-images::%s", imageName)

			stampy.Stamp(r.MetricsPath, "fissile", seriesName, "start")
			defer stampy.Stamp(r.MetricsPath, "fissile", seriesName, "done")
		}

		r.UI.Printf("Creating Dockerfile for release %s ...\n", color.YellowString(j.release.Name))
		dockerPopulator := r.NewDockerPopulator(j.release)

		if r.NoBuild {
			r.UI.Printf("Skipping build of release image %s because of flag\n", color.YellowString(j.release.Name))
			return nil
		}

		if r.OutputDirectory == "" {
			r.UI.Printf("Building docker image of %s...\n", color.YellowString(j.release.Name))

			var log bytes.Buffer
			stdoutWriter := docker.NewFormattingWriter(
				&log,
				docker.ColoredBuildStringFunc(imageName),
			)

			err := j.dockerManager.BuildImageFromCallback(imageName, stdoutWriter, dockerPopulator)
			if err != nil {
				log.WriteTo(r.UI)
				return fmt.Errorf("Error building image: %s", err.Error())
			}
		} else {
			r.UI.Printf("Building tarball of %s...\n", color.YellowString(j.release.Name))

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

// Build triggers the building of the release docker images in parallel.
func (r *ReleasesImageBuilder) Build(releases model.Releases) error {

	if r.WorkerCount < 1 {
		return fmt.Errorf("Invalid worker count %d", r.WorkerCount)
	}

	if r.OutputDirectory != "" {
		r.DockerRegistry = ""
		r.DockerOrganization = ""
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

	workerLib.MaxJobs = 1
	worker := workerLib.NewWorker()

	resultsCh := make(chan error)
	abort := make(chan struct{})
	for _, release := range releases {
		worker.Add(releaseBuildJob{
			release:       release,
			builder:       r,
			dockerManager: dockerManager,
			resultsCh:     resultsCh,
			abort:         abort,
		})
	}

	go worker.RunUntilDone()

	aborted := false
	for i := 0; i < len(releases); i++ {
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
