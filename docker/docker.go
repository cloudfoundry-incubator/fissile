package docker

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	dockerclient "github.com/fsouza/go-dockerclient"
)

const (
	// ContainerInPath is the input path for fissile
	ContainerInPath = "/fissile-in"
	// ContainerOutPath is the output path for fissile
	ContainerOutPath = "/fissile-out"
)

var (
	// ErrImageNotFound is the error returned when an image is not found.
	ErrImageNotFound = fmt.Errorf("Image not found")
)

// ImageManager handles Docker images
type ImageManager struct {
	client *dockerclient.Client
}

// NewImageManager creates an instance of ImageManager
func NewImageManager() (*ImageManager, error) {
	manager := &ImageManager{}

	client, err := dockerclient.NewClientFromEnv()
	manager.client = client

	if err != nil {
		return nil, err
	}

	return manager, nil
}

// StringFormatter is a formatting string function
type StringFormatter func(line string) string

// FormattingWriter wraps an io.WriteCloser so lines can be individually formatted.
type FormattingWriter struct {
	io.Writer
	io.Closer
	colorizer StringFormatter
	remainder *bytes.Buffer
	isClosed  bool
}

//NewFormattingWriter - Get a FormattingWriter here. aColorizer can be nil
func NewFormattingWriter(writer io.Writer, aColorizer StringFormatter) *FormattingWriter {
	return &FormattingWriter{
		Writer:    writer,
		colorizer: aColorizer,
		remainder: &bytes.Buffer{},
	}
}

func (w *FormattingWriter) Write(data []byte) (int, error) {
	if w.isClosed {
		return 0, fmt.Errorf("Attempt to write to a closed FormattingWriter")
	}
	lastEOL := bytes.LastIndex(data, []byte("\n"))
	if lastEOL == -1 {
		return w.remainder.Write(data)
	}
	defer func() {
		w.remainder.Reset()
		w.remainder.Write(data[lastEOL+1:])
	}()
	_, err := w.remainder.Write(data[0 : lastEOL+1])
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(w.remainder)
	for scanner.Scan() {
		_, err := fmt.Fprintln(w.Writer, w.color(scanner.Text()))
		if err != nil {
			return len(data), err
		}
	}
	return len(data), scanner.Err()
}

// Close ensures the remaining data is written to the io.Writer
func (w *FormattingWriter) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	if w.remainder.Len() == 0 {
		return nil
	}
	_, err := fmt.Fprintln(w.Writer, w.color(w.remainder.String()))
	return err
}

func (w *FormattingWriter) color(s string) string {
	if w.colorizer != nil {
		return w.colorizer(s)
	}
	return s
}

// BuildImage builds a docker image using a directory that contains a Dockerfile
func (d *ImageManager) BuildImage(dockerfileDirPath, name string, stdoutWriter io.WriteCloser) error {

	bio := dockerclient.BuildImageOptions{
		Name:         name,
		NoCache:      true,
		ContextDir:   filepath.Dir(dockerfileDirPath),
		OutputStream: stdoutWriter,
	}

	for _, envVar := range []string{"http_proxy", "https_proxy", "no_proxy"} {
		for _, name := range []string{strings.ToLower(envVar), strings.ToUpper(envVar)} {
			if val, ok := os.LookupEnv(name); ok {
				bio.BuildArgs = append(bio.BuildArgs, dockerclient.BuildArg{
					Name:  name,
					Value: val,
				})
			}
		}
	}

	if stdoutWriter != nil {
		defer func() {
			stdoutWriter.Close()
		}()
	}

	if err := d.client.BuildImage(bio); err != nil {
		return err
	}

	return nil
}

// FindImage will lookup an image in Docker
func (d *ImageManager) FindImage(imageName string) (*dockerclient.Image, error) {
	image, err := d.client.InspectImage(imageName)

	if err == dockerclient.ErrNoSuchImage {
		return nil, ErrImageNotFound
	} else if err != nil {
		return nil, fmt.Errorf("Error looking up image %s: %s", imageName, err.Error())
	}

	return image, nil
}

// HasImage determines if the given image already exists in Docker
func (d *ImageManager) HasImage(imageName string) bool {
	_, err := d.FindImage(imageName)
	return (err == nil)
}

// RemoveContainer will remove a container from Docker
func (d *ImageManager) RemoveContainer(containerID string) error {
	return d.client.RemoveContainer(dockerclient.RemoveContainerOptions{
		ID:    containerID,
		Force: true,
	})
}

// RemoveImage will remove an image from Docker's internal registry
func (d *ImageManager) RemoveImage(imageName string) error {
	return d.client.RemoveImage(imageName)
}

// CreateImage will create a Docker image
func (d *ImageManager) CreateImage(containerID string, repository string, tag string, message string, cmd []string) (*dockerclient.Image, error) {
	cco := dockerclient.CommitContainerOptions{
		Container:  containerID,
		Repository: repository,
		Tag:        tag,
		Author:     "fissile",
		Message:    message,
		Run: &dockerclient.Config{
			Cmd: cmd,
		},
	}

	return d.client.CommitContainer(cco)
}

// RunInContainerOpts encapsulates the options to RunInContainer()
type RunInContainerOpts struct {
	ContainerName string
	ImageName     string
	Cmd           []string
	// Mount points, src -> dest
	// dest may be special values ContainerInPath, ContainerOutPath
	Mounts map[string]string
	// Create local volumes.  Volumes are destroyed unless KeepContainer is true
	Volumes       map[string]map[string]string
	KeepContainer bool
	StdoutWriter  io.Writer
	StderrWriter  io.Writer
}

// RunInContainer will execute a set of commands within a running Docker container
func (d *ImageManager) RunInContainer(opts RunInContainerOpts) (exitCode int, container *dockerclient.Container, err error) {

	// Get current user info to map to container
	// os/user.Current() isn't supported when cross-compiling hence this code
	currentUID := syscall.Geteuid()
	currentGID := syscall.Getegid()
	var actualCmd, containerCmd []string
	if opts.KeepContainer {
		// Sleep effectively forever so if something goes wrong we can
		// docker exec -it bash into the container, investigate, and
		// manually kill the container. Most of the time the compile step
		// will succeed and the container will be killed and removed.
		containerCmd = []string{"sleep", "365d"}
		actualCmd = opts.Cmd
	} else {
		containerCmd = opts.Cmd
		// actualCmd not used
	}

	env := []string{
		fmt.Sprintf("HOST_USERID=%d", currentUID),
		fmt.Sprintf("HOST_USERGID=%d", currentGID),
	}
	for _, name := range []string{"http_proxy", "https_proxy"} {
		var proxyURL *url.URL
		var err error
		if val, ok := os.LookupEnv(name); ok {
			env = append(env, fmt.Sprintf("%s=%s", name, val))
			if proxyURL, err = url.Parse(val); err != nil {
				proxyURL = nil
			}
		}
		name = strings.ToUpper(name)
		if val, ok := os.LookupEnv(name); ok {
			env = append(env, fmt.Sprintf("%s=%s", name, val))
			if proxyURL == nil {
				// Follow curl, lower case env vars have precedence
				if proxyURL, err = url.Parse(val); err != nil {
					proxyURL = nil
				}
			}
		}
	}

	cco := dockerclient.CreateContainerOptions{
		Config: &dockerclient.Config{
			Tty:          false,
			AttachStdin:  false,
			AttachStdout: true,
			AttachStderr: true,
			Hostname:     "compiler",
			Domainname:   "fissile",
			Cmd:          containerCmd,
			WorkingDir:   "/",
			Image:        opts.ImageName,
			Env:          env,
		},
		HostConfig: &dockerclient.HostConfig{
			Privileged:     false,
			Binds:          []string{},
			ReadonlyRootfs: false,
		},
		Name: opts.ContainerName,
	}

	for name, dirverOpts := range opts.Volumes {
		name = fmt.Sprintf("volume_%s_%s", opts.ContainerName, name)
		_, err := d.client.CreateVolume(dockerclient.CreateVolumeOptions{
			Name:       name,
			DriverOpts: dirverOpts,
		})
		if err != nil {
			return -1, nil, err
		}
	}

	for src, dest := range opts.Mounts {
		if _, ok := opts.Volumes[src]; ok {
			// Attempt to mount a volume; use the generated name
			src = fmt.Sprintf("volume_%s_%s", opts.ContainerName, src)
		}
		mountString := fmt.Sprintf("%s:%s", src, dest)
		if dest == ContainerInPath {
			mountString += ":ro"
		}
		cco.HostConfig.Binds = append(cco.HostConfig.Binds, mountString)
	}

	container, err = d.client.CreateContainer(cco)
	if err != nil {
		return -1, nil, err
	}

	attached := make(chan struct{})

	attachCloseWaiter, attachErr := d.client.AttachToContainerNonBlocking(dockerclient.AttachToContainerOptions{
		Container: container.ID,

		InputStream:  nil,
		OutputStream: opts.StdoutWriter,
		ErrorStream:  opts.StderrWriter,

		Stdin:       false,
		Stdout:      opts.StdoutWriter != nil,
		Stderr:      opts.StderrWriter != nil,
		Stream:      true,
		RawTerminal: false,
		Success:     attached,
	})
	if attachErr != nil {
		return -1, container, fmt.Errorf("Error running in container: %s. Error attaching to container: %s", container.ID, attachErr.Error())
	}
	attached <- <-attached

	err = d.client.StartContainer(container.ID, container.HostConfig)
	if err != nil {
		return -1, container, err
	}

	closeFiles := func() {
		if stdoutCloser, ok := opts.StdoutWriter.(io.Closer); ok {
			stdoutCloser.Close()
		}
		if stderrCloser, ok := opts.StderrWriter.(io.Closer); ok {
			stderrCloser.Close()
		}
	}

	if !opts.KeepContainer {
		exitCode, err = d.client.WaitContainer(container.ID)
		attachCloseWaiter.Wait()
		closeFiles()
		if err != nil {
			exitCode = -1
		}
		return exitCode, container, nil
	}
	// KeepContainer mode:
	// Run the cmd with 'docker exec ...' so we can keep the container around.
	// Note that this time we'll need to stop it if it doesn't fail
	cmdArgs := append([]string{"exec", "-i", container.ID}, actualCmd...)

	// Couldn't get this to work with dockerclient.Exec, so do it this way
	execCmd := exec.Command("docker", cmdArgs...)
	execCmd.Stdout = opts.StdoutWriter
	execCmd.Stderr = opts.StderrWriter
	err = execCmd.Run()
	// No need to wait on execCmd or on attachCloseWaiter
	if err == nil {
		exitCode = 0
	} else {
		exitCode = -1
	}
	closeFiles()
	return exitCode, container, err
}

// RemoveVolumes removes any temporary volumes assoicated with a container
func (d *ImageManager) RemoveVolumes(container *dockerclient.Container) error {
	volumes, err := d.client.ListVolumes(dockerclient.ListVolumesOptions{})
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf("volume_%s", strings.TrimLeft(container.Name, "/"))

	// Sadly, both container.Volumes and container.VolumesRW are empty?
	for _, volume := range volumes {
		if strings.HasPrefix(volume.Name, prefix) {
			if err := d.client.RemoveVolume(volume.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// ColoredBuildStringFunc returns a formatting function for colorizing strings.
func ColoredBuildStringFunc(buildName string) StringFormatter {
	return func(s string) string {
		return color.GreenString("build-%s > %s", color.MagentaString(buildName), color.WhiteString("%s", s))
	}
}
