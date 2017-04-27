package docker

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/mock/gomock"
	"github.com/hpcloud/fissile/util"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	dockerImageEnvVar      = "FISSILE_TEST_DOCKER_IMAGE"
	defaultDockerTestImage = "ubuntu:14.04"
)

var dockerImageName string

func TestMain(m *testing.M) {
	dockerImageName = os.Getenv(dockerImageEnvVar)
	if dockerImageName == "" {
		dockerImageName = defaultDockerTestImage
	}

	retCode := m.Run()

	os.Exit(retCode)
}

func TestFindImageOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.NoError(err)

	image, err := dockerManager.FindImage(dockerImageName)

	if !assert.NoError(err) {
		return
	}
	assert.NotEmpty(image.ID)
}

func TestFindImageNotOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.NoError(err)

	name := uuid.New()
	_, err = dockerManager.FindImage(name)

	assert.Error(err)
	assert.Equal(ErrImageNotFound, err)
}

func TestHasImageOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.NoError(err)

	assert.True(dockerManager.HasImage(dockerImageName))
}

func TestHasImageNotOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.NoError(err)

	name := uuid.New()
	assert.False(dockerManager.HasImage(name))
}

func TestRunInContainer(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)

	stdoutWriter := &bytes.Buffer{}
	stderrWriter := &bytes.Buffer{}

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"hostname"},
		StdoutWriter:  stdoutWriter,
		StderrWriter:  stderrWriter,
	})

	if !assert.NoError(err) {
		return
	}
	assert.Equal(0, exitCode)
	assert.Equal("compiler.fissile\n", stdoutWriter.String())
	assert.Empty(strings.TrimSpace(stderrWriter.String()))

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)
}

func TestRunInContainerStderr(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)

	buf2 := new(bytes.Buffer)
	stdoutWriter := NewFormattingWriter(buf2, nil)
	buf := new(bytes.Buffer)
	stderrWriter := NewFormattingWriter(buf, nil)

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"ping", "-foo"},
		StdoutWriter:  stdoutWriter,
		StderrWriter:  stderrWriter,
	})

	if !assert.NoError(err) {
		return
	}
	assert.Equal(2, exitCode)
	assert.NotEmpty(buf)
	assert.Contains(buf.String(), "invalid option")

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)
}

func TestRunInContainerWithInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)

	buf := new(bytes.Buffer)
	stdoutWriter := NewFormattingWriter(buf, nil)
	stderrWriter := new(bytes.Buffer)

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"ls", ContainerInPath},
		Mounts:        map[string]string{"/": ContainerInPath},
		StdoutWriter:  stdoutWriter,
		StderrWriter:  stderrWriter,
	})

	if !assert.NoError(err) {
		return
	}
	assert.Equal(0, exitCode)
	assert.NotEmpty(buf)
	assert.Empty(strings.TrimSpace(stderrWriter.String()))

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)
}

func TestRunInContainerWithReadOnlyInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"touch", filepath.Join(ContainerInPath, "fissile-test.txt")},
		Mounts:        map[string]string{"/": ContainerInPath},
	})

	if !assert.NoError(err) {
		return
	}
	assert.NotEqual(0, exitCode)

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)
}

func TestRunInContainerWithOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)

	buf := new(bytes.Buffer)
	stdoutWriter := NewFormattingWriter(buf, nil)

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"ls", ContainerOutPath},
		Mounts:        map[string]string{"/tmp": ContainerOutPath},
		StdoutWriter:  stdoutWriter,
	})

	if !assert.NoError(err) {
		return
	}
	assert.Equal(0, exitCode)
	assert.NotEmpty(buf)

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)
}

func TestRunInContainerWithWritableOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"touch", filepath.Join(ContainerOutPath, "fissile-test.txt")},
		Mounts:        map[string]string{"/tmp": ContainerOutPath},
	})

	if !assert.NoError(err) {
		return
	}
	assert.Equal(0, exitCode)

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)
}

func TestRunInContainerVolumeRemoved(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.NoError(err)

	volumeName := uuid.New()
	stdoutWriter := &bytes.Buffer{}

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"cat", "/proc/self/mounts"},
		Mounts:        map[string]string{volumeName: ContainerOutPath},
		Volumes:       map[string]map[string]string{volumeName: nil},
		StdoutWriter:  stdoutWriter,
	})

	if !assert.NoError(err) {
		return
	}
	assert.Equal(0, exitCode)

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)

	err = dockerManager.RemoveVolumes(container)
	assert.NoError(err)

	assert.Contains(stdoutWriter.String(), ContainerOutPath)

	volumes, err := dockerManager.client.ListVolumes(dockerclient.ListVolumesOptions{})
	if assert.NoError(err) {
		for _, volume := range volumes {
			if !assert.NotContains(volume.Name, volumeName) {
				// If the test fails, we should attempt to clean up anyway
				err = dockerManager.client.RemoveVolume(volume.Name)
				assert.NoError(err)
			}
		}
	}
}

func TestCreateImageOk(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)

	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: getTestName(),
		ImageName:     dockerImageName,
		Cmd:           []string{"ping", "127.0.0.1", "-c", "1"},
	})

	if !assert.NoError(err) {
		return
	}
	assert.Equal(0, exitCode)

	testRepo := getTestName()
	testTag := getTestName()

	image, err := dockerManager.CreateImage(
		container.ID,
		testRepo,
		testTag,
		"fissile-test",
		[]string{"ping", "127.0.0.1", "-c", "1"},
	)

	if !assert.NoError(err) {
		return
	}
	assert.NotEmpty(image.ID)

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)

	err = dockerManager.RemoveImage(fmt.Sprintf("%s:%s", testRepo, testTag))
	assert.NoError(err)
}

func TestVerifySuccessfulDebugContainerStays(t *testing.T) {
	verifyDebugContainerStays(t, true)
}

func TestVerifyFailedDebugContainerStays(t *testing.T) {
	verifyDebugContainerStays(t, false)
}

func verifyDebugContainerStays(t *testing.T, cmdShouldSucceed bool) {

	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.NoError(err)
	testName := getTestName()

	// Run /bin/true to succeed, /bin/false to fail
	exitCode, container, err := dockerManager.RunInContainer(RunInContainerOpts{
		ContainerName: testName,
		ImageName:     dockerImageName,
		Cmd:           []string{fmt.Sprintf("/bin/%t", cmdShouldSucceed)},
		KeepContainer: true,
	})
	if cmdShouldSucceed {
		if !assert.NoError(err) {
			return
		}
		assert.Equal(0, exitCode)
	} else {
		if !assert.Error(err) {
			return
		}
		assert.Equal(-1, exitCode)
	}

	// Run ps to get the values
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}::{{.ID}}::{{.Command}}", "--no-trunc")
	output, err := cmd.CombinedOutput()
	if !assert.NoError(err) {
		return
	}
	outputLines := strings.Split(string(output), "\n")
	wantedOutputLine := ""
	for _, s := range outputLines {
		if strings.Index(s, container.ID) >= 0 {
			assert.Empty(wantedOutputLine, fmt.Sprintf("Found multiple hits for a running container: %s", container.ID))
			wantedOutputLine = s
		}
	}
	assert.NotEmpty(wantedOutputLine, fmt.Sprintf("Didn't find a hit for running container: %s", container.ID))
	if wantedOutputLine != "" {
		parts := strings.Split(wantedOutputLine, "::")
		assert.Len(parts, 3, fmt.Sprintf("Splitting up '%s' => %d parts", wantedOutputLine, len(parts)))
		assert.Equal(testName, parts[0], wantedOutputLine)
		assert.Equal("\"sleep 365d\"", parts[2])
	}

	err = dockerManager.RemoveContainer(container.ID)
	assert.NoError(err)

	// Make sure the container is gone now
	// Run ps to get the values
	cmd = exec.Command("docker", "ps", "--format", "{{.ID}}:", "--no-trunc")
	output, err = cmd.CombinedOutput()
	if !assert.NoError(err) {
		return
	}
	assert.Equal(-1, strings.Index(string(output), container.ID), "Found container %+v in %+v", container.ID, string(output))
}

func getTestName() string {
	return fmt.Sprintf("fissile-test-%s", uuid.New())
}

func verifyWriteOutput(t *testing.T, inputs ...string) {

	assert := assert.New(t)

	fullInput := strings.Join(inputs, "")
	expected := strings.Split(strings.TrimSuffix(fullInput, "\n"), "\n")
	expectedAmount := len(fullInput)

	buf := &bytes.Buffer{}
	writer := NewFormattingWriter(buf, func(line string) string {
		return fmt.Sprintf(">>>%s<<<", strings.TrimSuffix(line, "\n"))
	})

	totalWritten := 0
	for _, input := range inputs {
		amountWritten, err := writer.Write([]byte(input))
		assert.NoError(err)
		totalWritten += amountWritten
	}
	writer.Close()
	assert.Equal(fmt.Sprintf(">>>%s<<<\n", strings.Join(expected, "<<<\n>>>")),
		buf.String(), "Unexpected data written for %#v", inputs)
	assert.Equal(expectedAmount, totalWritten, "Unexpected amount written for %#v", inputs)
}

func TestFormatWriterOneLine(t *testing.T) {
	verifyWriteOutput(t, "hello\n")
	verifyWriteOutput(t, "hello\nworld\n")
	verifyWriteOutput(t, "hello")
	verifyWriteOutput(t, "aaa\nbbb\nccc")
	verifyWriteOutput(t, "multipl", "e\ncalls")
	verifyWriteOutput(t, "multipl", "e\ncalls\n")
}

func doTestBuildImageFromCallback(t *testing.T, callback func(*tar.Writer) error, postRun func(error, *ImageManager, string)) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.NoError(err)

	imageName := uuid.New()
	hasImage, err := dockerManager.HasImage(imageName)
	if assert.NoError(err) {
		assert.False(hasImage, "Failed to get an unused image name")
	}

	err = dockerManager.BuildImageFromCallback(imageName, ioutil.Discard, callback)
	postRun(err, dockerManager, imageName)
}

func TestBuildImageFromCallback(t *testing.T) {
	assert := assert.New(t)
	doTestBuildImageFromCallback(t, func(tarStream *tar.Writer) error {
		contents := bytes.NewBufferString("FROM scratch\nENV hello=world")
		header := tar.Header{
			Name:     "Dockerfile",
			Mode:     0644,
			Size:     int64(contents.Len()),
			Typeflag: tar.TypeReg,
		}
		assert.NoError(tarStream.WriteHeader(&header))
		_, err := io.Copy(tarStream, contents)
		assert.NoError(err)
		return nil
	}, func(err error, dockerManager *ImageManager, imageName string) {
		if !assert.NoError(err) {
			return
		}
		hasImage, err := dockerManager.HasImage(imageName)
		if assert.NoError(err) {
			assert.True(hasImage, "Image did not build")
		}
		err = dockerManager.RemoveImage(imageName)
		assert.NoError(err, "Failed to remove image %s", imageName)
		hasImage, err = dockerManager.HasImage(imageName)
		if assert.NoError(err) {
			assert.False(hasImage, "Failed to remove image")
		}
	})
}

func TestBuildImageFromCallbackCallbackFailure(t *testing.T) {
	assert := assert.New(t)
	doTestBuildImageFromCallback(t, func(tarStream *tar.Writer) error {
		err := util.WriteToTarStream(tarStream, []byte("FROM scratch\nENV hello=world"), tar.Header{
			Name: "Dockerfile",
		})
		assert.NoError(err)
		return fmt.Errorf("Dummy error")
	}, func(err error, dockerManager *ImageManager, imageName string) {
		if assert.Error(err) {
			assert.EqualError(err, "Dummy error", "Error message should be from callback")
		}
		// The image _could_ have been built, since nothing in it is missing
		hasImage, err := dockerManager.HasImage(imageName)
		if assert.NoError(err) && hasImage {
			assert.NoError(dockerManager.RemoveImage(imageName))
		}
	})
}

func TestBuildImageFromCallbackDockerFailure(t *testing.T) {
	assert := assert.New(t)
	doTestBuildImageFromCallback(t, func(*tar.Writer) error {
		// We don't have a Dockerfile, it should fail to build
		return nil
	}, func(err error, dockerManager *ImageManager, imageName string) {
		if assert.Error(err, "Image should have failed to build") {
			assert.Contains(err.Error(), "Cannot locate specified Dockerfile")
		}
		hasImage, err := dockerManager.HasImage(imageName)
		assert.NoError(err)
		assert.False(hasImage, "Image %s should not be available", imageName)
	})
}

//go:generate -command mockgen go run ../vendor/github.com/golang/mock/mockgen/mockgen.go ../vendor/github.com/golang/mock/mockgen/parse.go ../vendor/github.com/golang/mock/mockgen/reflect.go
//go:generate mockgen -source docker.go -package docker -destination mock_dockerclient_generated.go dockerClient

type mockImage struct {
	name    string
	labels  map[string]string
	history []dockerclient.ImageHistory
}

func setupFindBestImageWithLabels(mock *MockdockerClient, data []mockImage) {
	var laterImages []dockerclient.APIImages
	for _, image := range data {
		mock.EXPECT().
			ImageHistory(image.name).
			Return(image.history, nil)
		if image.name != data[0].name {
			apiImage := dockerclient.APIImages{
				ID:     image.history[0].ID,
				Size:   image.history[0].Size,
				Labels: image.labels,
			}
			laterImages = append(laterImages, apiImage)
		}
	}
	expectedListOptions := dockerclient.ListImagesOptions{
		All:     true,
		Filters: map[string][]string{"since": []string{data[0].name}},
	}
	mock.EXPECT().
		ListImages(expectedListOptions).
		Return(laterImages, nil)
	mock.EXPECT().
		ListImages(dockerclient.ListImagesOptions{Filter: data[0].name}).
		Return([]dockerclient.APIImages{
			dockerclient.APIImages{ID: data[0].history[0].ID},
		}, nil)
}

func TestFindBestImageWithLabels_OnlyBase(t *testing.T) {
	assert := assert.New(t)
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockDockerClient := NewMockdockerClient(mockCtl)
	dockerManager := &ImageManager{
		client: mockDockerClient,
	}

	images := []mockImage{
		{
			name: "base-image:tag",
			history: []dockerclient.ImageHistory{
				{ID: "base-image-id"},
			},
		},
	}
	setupFindBestImageWithLabels(mockDockerClient, images)

	wantedTags := []string{"wanted-tag"} // There is no match here
	desiredImage, foundLabels, err := dockerManager.FindBestImageWithLabels(images[0].name, wantedTags)
	assert.NoError(err)
	assert.Equal(images[0].history[0].ID, desiredImage)
	assert.Empty(foundLabels)
}

func TestFindBestImageWithLabels_Simple(t *testing.T) {
	assert := assert.New(t)
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockDockerClient := NewMockdockerClient(mockCtl)
	dockerManager := &ImageManager{
		client: mockDockerClient,
	}

	wantedTag := "wanted-tag"
	images := []mockImage{
		{
			name: "base-image:tag",
			history: []dockerclient.ImageHistory{
				{ID: "base-image-id"},
			},
		},
		{
			name: "some-other-layer",
			history: []dockerclient.ImageHistory{
				{ID: "some-other-layer", Size: 1},
				{ID: "base-image-id"},
			},
			labels: map[string]string{wantedTag: "value"},
		},
	}
	setupFindBestImageWithLabels(mockDockerClient, images)

	desiredImage, foundLabels, err := dockerManager.FindBestImageWithLabels(images[0].name, []string{wantedTag})
	assert.NoError(err)
	assert.Equal(images[1].history[0].ID, desiredImage)
	assert.Equal(images[1].labels, foundLabels)
}

func TestFindBestImageWithLabels_PickSmaller(t *testing.T) {
	assert := assert.New(t)
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockDockerClient := NewMockdockerClient(mockCtl)
	dockerManager := &ImageManager{
		client: mockDockerClient,
	}

	wantedTag := "wanted-tag"
	images := []mockImage{
		{
			name: "base-image:tag",
			history: []dockerclient.ImageHistory{
				{ID: "base-image-id"},
			},
		},
		{
			name: "some-other-layer",
			history: []dockerclient.ImageHistory{
				{ID: "some-other-layer", Size: 2},
				{ID: "base-image-id"},
			},
			labels: map[string]string{wantedTag: "value"},
		},
		{
			name: "some-third-layer",
			history: []dockerclient.ImageHistory{
				{ID: "some-third-layer", Size: 1},
				{ID: "base-image-id"},
			},
			labels: map[string]string{wantedTag: "other-value"},
		},
	}
	setupFindBestImageWithLabels(mockDockerClient, images)

	desiredImage, foundLabels, err := dockerManager.FindBestImageWithLabels(images[0].name, []string{wantedTag})
	assert.NoError(err)
	assert.Equal(images[2].history[0].ID, desiredImage)
	assert.Equal(images[2].labels, foundLabels)
}

func TestFindBestImageWithLabels_PickMostMatchingTags(t *testing.T) {
	assert := assert.New(t)
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockDockerClient := NewMockdockerClient(mockCtl)
	dockerManager := &ImageManager{
		client: mockDockerClient,
	}

	wantedTags := []string{"tag-one", "tag-two"}
	images := []mockImage{
		{
			name: "base-image:tag",
			history: []dockerclient.ImageHistory{
				{ID: "base-image-id"},
			},
		},
		{
			name: "some-other-layer",
			history: []dockerclient.ImageHistory{
				{ID: "some-other-layer", Size: 2},
				{ID: "base-image-id"},
			},
			labels: map[string]string{"tag-one": "1", "tag-two": "2"},
		},
		{
			name: "some-third-layer",
			history: []dockerclient.ImageHistory{
				{ID: "some-third-layer", Size: 1},
				{ID: "base-image-id"},
			},
			labels: map[string]string{"tag-one": "1"},
		},
	}
	setupFindBestImageWithLabels(mockDockerClient, images)

	desiredImage, foundLabels, err := dockerManager.FindBestImageWithLabels(images[0].name, wantedTags)
	assert.NoError(err)
	assert.Equal(images[1].history[0].ID, desiredImage)
	assert.Equal(images[1].labels, foundLabels)
}
