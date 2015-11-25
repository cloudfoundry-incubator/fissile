package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/util"

	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

func TestGenerateRoleImageDockerfile(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		os.Stdin,
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.Nil(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14.15", "6.28.30", ui)

	dockerfileContents, err := roleImageBuilder.generateDockerfile(rolesManifest.Roles[0])
	assert.Nil(err)

	dockerfileString := string(dockerfileContents)
	assert.Contains(dockerfileString, "foo-role-base:6.28.30")
	assert.Contains(dockerfileString, "ADD LICENSE.md  /opt/hcf/share/doc")
}

func TestGenerateRoleImageRunScript(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		os.Stdin,
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.Nil(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14.15", "6.28.30", ui)

	runScriptContents, err := roleImageBuilder.generateRunScript(rolesManifest.Roles[0])
	assert.Nil(err)
	assert.Contains(string(runScriptContents), "/var/vcap/jobs-src/tor/templates/data/properties.sh.erb")
	assert.Contains(string(runScriptContents), "/opt/hcf/monitrc.erb")
	assert.Contains(string(runScriptContents), "/opt/hcf/startup/myrole.sh")
	assert.Contains(string(runScriptContents), "monit -vI")

	runScriptContents, err = roleImageBuilder.generateRunScript(rolesManifest.Roles[1])
	assert.Nil(err)
	assert.NotContains(string(runScriptContents), "monit -vI")
	assert.NotContains(string(runScriptContents), "/etc/monitrc")
	assert.Contains(string(runScriptContents), "/var/vcap/jobs/tor/bin/run")
}

func TestGenerateRoleImageDockerfileDir(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		os.Stdin,
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.Nil(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14.15", "6.28.30", ui)

	dockerfileDir, err := roleImageBuilder.CreateDockerfileDir(rolesManifest.Roles[0])
	assert.Nil(err)

	assert.Equal(filepath.Join(targetPath, "myrole"), dockerfileDir)

	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole"), true, "role dir"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "LICENSE.md"), false, "license file"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "run.sh"), false, "run script"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "Dockerfile"), false, "Dockerfile"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "packages", "tor"), true, "package dir"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "packages", "tor", "bar"), false, "compilation artifact"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "jobs", "tor", "monit"), false, "job monit file"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "jobs", "tor", "templates", "bin", "monit_debugger"), false, "job template file"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "role-startup"), true, "role startup scripts dir"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "role-startup", "myrole.sh"), false, "role specific startup script"))

	// job.MF should not be there
	assert.NotNil(util.ValidatePath(filepath.Join(targetPath, "myrole", "jobs", "tor", "job.MF"), false, "job manifest file"))
}

type buildImageCallback func(name string) error

type mockDockerImageBuilder struct {
	callback buildImageCallback
}

func (m *mockDockerImageBuilder) BuildImage(dockerDirPath, name string, stdoutProcessor docker.ProcessOutStream) error {
	return m.callback(name)
}

func TestBuildRoleImages(t *testing.T) {

	origNewDockerImageBuilder := newDockerImageBuilder
	defer func() {
		newDockerImageBuilder = origNewDockerImageBuilder
	}()

	type dockerBuilderMock struct {
	}

	mockBuilder := mockDockerImageBuilder{}
	newDockerImageBuilder = func() (dockerImageBuilder, error) {
		return &mockBuilder, nil
	}

	assert := assert.New(t)

	ui := termui.New(
		os.Stdin,
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.Nil(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder(
		"test-repository",
		compiledPackagesDir,
		targetPath,
		"http://127.0.0.1:8500",
		"hcf",
		"3.14.15",
		"6.28.30",
		ui,
	)

	// Check that making the first wait for the second job works
	secondJobReady := make(chan struct{})
	mockBuilder.callback = func(name string) error {
		if strings.Contains(name, "-myrole:") {
			<-secondJobReady
			return nil
		}
		if strings.Contains(name, "-foorole:") {
			close(secondJobReady)
			return nil
		}
		t.Errorf("Got unexpected job %s", name)
		return fmt.Errorf("Unknown docker image name %s", name)
	}

	err = roleImageBuilder.BuildRoleImages(
		rolesManifest.Roles,
		"test-repository",
		"3.14.15",
		false,
		2,
	)
	assert.Nil(err)

	err = os.RemoveAll(targetPath)
	assert.Nil(err, "Failed to remove target")

	// Should not allow invalid worker counts
	err = roleImageBuilder.BuildRoleImages(
		rolesManifest.Roles,
		"test-repository",
		"3.14.15",
		false,
		0,
	)
	assert.NotNil(err, "Invalid worker count should result in an error")
	assert.Contains(err.Error(), "count", "Building the image should have failed due to invalid worker count")

	// Check that failing the first job will not run the second job
	hasRunSecondJob := false
	mockBuilder.callback = func(name string) error {
		if strings.Contains(name, "-myrole:") {
			return fmt.Errorf("Deliberate failure")
		}
		if strings.Contains(name, "-foorole:") {
			assert.False(hasRunSecondJob, "Second job should not run if first job failed")
			hasRunSecondJob = true
		}
		t.Errorf("Got unexpected job %s", name)
		return fmt.Errorf("Unknown docker image name %s", name)
	}

	err = roleImageBuilder.BuildRoleImages(
		rolesManifest.Roles,
		"test-repository",
		"3.14.15",
		false,
		1,
	)
	assert.Contains(err.Error(), "Deliberate failure", "Returned error should be from first job failing")
	assert.False(hasRunSecondJob, "Second job should not have run")
}
