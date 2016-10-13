package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dockerclient "github.com/fsouza/go-dockerclient"
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
		assert.Equal(3, len(parts), fmt.Sprintf("Splitting up '%s' => %d parts", wantedOutputLine, len(parts)))
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
