package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
	assert.Nil(err)

	image, err := dockerManager.FindImage(dockerImageName)

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.NotEmpty(image.ID)
}

func TestShowImageNotOK(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()
	assert.Nil(err)

	name := uuid.New()
	_, err = dockerManager.FindImage(name)

	assert.NotNil(err)
	assert.Equal(ErrImageNotFound, err)
}

func TestRunInContainer(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ping", "127.0.0.1", "-c", "1"},
		"",
		"",
		false,
		func(stdout io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stdout)
			output = buf.String()
		},
		nil,
	)

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.Equal(0, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerStderr(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ping", "-foo"},
		"",
		"",
		false,
		nil,
		func(stderr io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stderr)
			output = buf.String()
		},
	)

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.Equal(2, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ls", ContainerInPath},
		"/",
		"",
		false,
		func(stdout io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stdout)
			output = buf.String()
		},
		nil,
	)

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.Equal(0, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithReadOnlyInFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"touch", filepath.Join(ContainerInPath, "fissile-test.txt")},
		"/",
		"",
		false,
		nil,
		nil,
	)

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.NotEqual(0, exitCode)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	var output string

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ls", ContainerOutPath},
		"",
		"/tmp",
		false,
		func(stdout io.Reader) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(stdout)
			output = buf.String()
		},
		nil,
	)

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.Equal(0, exitCode)
	assert.NotEmpty(output)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestRunInContainerWithWritableOutFiles(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"touch", filepath.Join(ContainerOutPath, "fissile-test.txt")},
		"",
		"/tmp",
		false,
		nil,
		nil,
	)

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.Equal(0, exitCode)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)
}

func TestCreateImageOk(t *testing.T) {
	assert := assert.New(t)

	dockerManager, err := NewImageManager()

	assert.Nil(err)

	exitCode, container, err := dockerManager.RunInContainer(
		getTestName(),
		dockerImageName,
		[]string{"ping", "127.0.0.1", "-c", "1"},
		"",
		"",
		false,
		nil,
		nil,
	)

	assert.Nil(err)
	if err != nil {
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

	assert.Nil(err)
	if err != nil {
		return
	}
	assert.NotEmpty(image.ID)

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)

	err = dockerManager.RemoveImage(fmt.Sprintf("%s:%s", testRepo, testTag))
	assert.Nil(err)
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

	assert.Nil(err)
	testName := getTestName()

	// Run /bin/true to succeed, /bin/false to fail
	exitCode, container, err := dockerManager.RunInContainer(
		testName,
		dockerImageName,
		[]string{fmt.Sprintf("/bin/%t", cmdShouldSucceed)},
		"",
		"",
		true,
		nil,
		nil,
	)
	if cmdShouldSucceed {
		assert.Nil(err)
		if err != nil {
			return
		}
		assert.Equal(0, exitCode)
	} else {
		assert.NotNil(err)
		if err == nil {
			return
		}
		assert.Equal(-1, exitCode)
	}

	// Run ps to get the values
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}::{{.ID}}::{{.Command}}", "--no-trunc")
	output, err := cmd.CombinedOutput()
	assert.Nil(err)
	if err != nil {
		return
	}
	outputLines := strings.Split(string(output), "\n")
	wantedOutputLine := ""
	for _, s := range outputLines {
		if strings.Index(s, container.ID) >= 0 {
			assert.Equal("", wantedOutputLine, fmt.Sprintf("Found multiple hits for a running container: %s", container.ID))
			wantedOutputLine = s
		}
	}
	assert.NotEqual("", wantedOutputLine, fmt.Sprintf("Didn't find a hit for running container: %s", container.ID))
	if wantedOutputLine != "" {
		parts := strings.Split(wantedOutputLine, "::")
		assert.Equal(3, len(parts), fmt.Sprintf("Splitting up '%s' => %d parts", wantedOutputLine, len(parts)))
		assert.Equal(testName, parts[0], wantedOutputLine)
		assert.Equal("\"sleep 365d\"", parts[2])
	}

	err = dockerManager.RemoveContainer(container.ID)
	assert.Nil(err)

	// Make sure the container is gone now
	// Run ps to get the values
	cmd = exec.Command("docker", "ps", "--format", "{{.ID}}:", "--no-trunc")
	output, err = cmd.CombinedOutput()
	assert.Nil(err)
	if err != nil {
		return
	}
	assert.Equal(-1, strings.Index(string(output), container.ID))
}

func getTestName() string {
	return fmt.Sprintf("fissile-test-%s", uuid.New())
}
