package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// dockerClient is an interface to represent a dockerclient.Client
// It exists so we can replace it with a mock object in tests
type dockerClient interface {
	AttachToContainerNonBlocking(dockerclient.AttachToContainerOptions) (dockerclient.CloseWaiter, error)
	BuildImage(dockerclient.BuildImageOptions) error
	CommitContainer(dockerclient.CommitContainerOptions) (*dockerclient.Image, error)
	CreateContainer(dockerclient.CreateContainerOptions) (*dockerclient.Container, error)
	CreateVolume(dockerclient.CreateVolumeOptions) (*dockerclient.Volume, error)
	ImageHistory(string) ([]dockerclient.ImageHistory, error)
	InspectImage(string) (*dockerclient.Image, error)
	ListImages(dockerclient.ListImagesOptions) ([]dockerclient.APIImages, error)
	ListVolumes(dockerclient.ListVolumesOptions) ([]dockerclient.Volume, error)
	RemoveContainer(dockerclient.RemoveContainerOptions) error
	RemoveImage(string) error
	RemoveVolume(string) error
	StartContainer(string, *dockerclient.HostConfig) error
	WaitContainer(string) (int, error)
}

// ImageManager handles Docker images
type ImageManager struct {
	client dockerClient
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

// BuildImageFromCallback builds a docker image by letting a callback populate
// a tar.Writer; the callback must write a Dockerfile into the tar stream (as
// well as any additional build context).  If stdoutWriter implements io.Closer,
// it will be closed when done.
func (d *ImageManager) BuildImageFromCallback(name string, stdoutWriter io.Writer, callback func(*tar.Writer) error) error {
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		return err
	}

	bio := dockerclient.BuildImageOptions{
		Name:         name,
		NoCache:      true,
		InputStream:  pipeReader,
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

	if stdoutCloser, ok := stdoutWriter.(io.Closer); ok {
		defer func() {
			stdoutCloser.Close()
		}()
	}

	writerErrorChan := make(chan error, 1)
	go func() {
		defer close(writerErrorChan)
		defer pipeWriter.Close()
		tarWriter := tar.NewWriter(pipeWriter)
		var err error
		if err = callback(tarWriter); err == nil {
			err = tarWriter.Close()
		}
		writerErrorChan <- err
	}()

	err = d.client.BuildImage(bio)

	// Prefer returning the error from the tar writer; that normally
	// has more useful details.
	if writerErr := <-writerErrorChan; writerErr != nil {
		return writerErr
	}

	return err
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

// FindBestImageWithLabels finds the best image that has a given base
// image, and has as many of the given labels as possible.  Returns
// the best matching image name, and all of the matched labels (and
// their values). Manadatory labels are labels an image must have to
// be considered as candidate.
func (d *ImageManager) FindBestImageWithLabels(baseImageName string, labels []string, mandatory []string) (string, map[string]string, error) {
	// We want to walk through all images newer than the provided base image,
	// and find everything with some set of matching labels.  For all of the
	// images with at least one match, we use the smallest-sized image for each
	// unique combination of labels (i.e. discard images that had additional
	// unrelated labels on top).  We then want to use the largest-sized image
	// in the hopes of reducing copies from unchanged packages.
	// Note that this means the _number_ of matching labels don't matter as long
	// as it's at least one.

	// Get information about the image we have first
	history, err := d.client.ImageHistory(baseImageName)
	if err != nil {
		return "", nil, err
	}
	if len(history) < 1 {
		return "", nil, fmt.Errorf("Image %s has no history", baseImageName)
	}
	desiredLayer := history[0].ID

	// Convert the desired labels to a hash for easier lookup
	desiredLabels := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		desiredLabels[label] = struct{}{}
	}

	// Convert the mandatory labels to a hash for easier lookup and matching
	mandatoryLabels := make(map[string]struct{}, len(labels))
	for _, label := range mandatory {
		mandatoryLabels[label] = struct{}{}
	}

	// Iterate through all available images and find candidates
	matchingImages := make(map[string]dockerclient.APIImages)
	listOptions := dockerclient.ListImagesOptions{
		All:     true,
		Filters: map[string][]string{"since": []string{baseImageName}},
	}
	candidates, err := d.client.ListImages(listOptions)
	if err != nil {
		return "", nil, err
	}
	for _, candidate := range candidates {
		history, err = d.client.ImageHistory(candidate.ID)
		if err != nil {
			return "", nil, err
		}
		found := false
		for _, layer := range history {
			if layer.ID == desiredLayer {
				found = true
				break
			}
		}
		if !found {
			// This image does not derive from the desired base image
			continue
		}

		if !d.HasLabels(&candidate, mandatoryLabels) {
			// This image does not have all of the mandatory labels
			continue
		}

		// Figure out how many labels we match and put it in the list
		var matchedLabels []string
		for label := range candidate.Labels {
			if _, ok := desiredLabels[label]; ok {
				matchedLabels = append(matchedLabels, label)
			}
		}
		if len(matchedLabels) == 0 {
			// This is no better than the base image
			continue
		}
		sort.Strings(matchedLabels)
		matchKey := strings.Join(matchedLabels, "\n")
		oldMatch, ok := matchingImages[matchKey]
		if !ok {
			// No previous match, this is the best so far
			matchingImages[matchKey] = candidate
			continue
		}
		if oldMatch.Size > candidate.Size {
			// The new candidate matches all the labels of the old one,
			// but is smaller
			matchingImages[matchKey] = candidate
		} else if oldMatch.Size == candidate.Size {
			// The images have the same size; this can happen if there are
			// additional metadata steps (e.g. LABEL, ENTRYPOINT).  We want to
			// have as few layers as possible, though.  As a proxy, use the
			// older image by creation date; if one is an ancestor of the other,
			// this will get us the image with fewer layers.
			if oldMatch.Created > candidate.Created {
				matchingImages[matchKey] = candidate
			}
		}
	}

	// Find the largest match
	listOptions = dockerclient.ListImagesOptions{Filter: baseImageName}
	baseImages, err := d.client.ListImages(listOptions)
	if err != nil {
		return "", nil, err
	}
	bestMatch := baseImages[0]
	for _, candidate := range matchingImages {
		if candidate.Size > bestMatch.Size {
			bestMatch = candidate
		}
	}

	// Find the matching labels
	matchedLabels := make(map[string]string)
	for _, label := range labels {
		if value, ok := bestMatch.Labels[label]; ok {
			matchedLabels[label] = value
		}
	}
	// Include the mandatory labels too.
	for _, label := range mandatory {
		if value, ok := bestMatch.Labels[label]; ok {
			matchedLabels[label] = value
		}
	}

	return bestMatch.ID, matchedLabels, nil
}

// HasLabels determines if all of the provided labels (keys of the
// map) are in the set of the image's labels. It returns true if so,
// and false otherwise.
func (d *ImageManager) HasLabels(image *dockerclient.APIImages, labels map[string]struct{}) bool {
	var found int
	// Check that all labels are among the image's labels
	for label := range image.Labels {
		if _, ok := labels[label]; ok {
			found = found + 1
		}
	}
	return found == len(labels)
}

// HasImage determines if the given image already exists in Docker
func (d *ImageManager) HasImage(imageName string) (bool, error) {
	if _, err := d.FindImage(imageName); err == ErrImageNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
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
	NetworkMode   string
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
			NetworkMode:    opts.NetworkMode,
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
				err = fmt.Errorf("Volume %s: %s", volume.Name, err.Error())
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
