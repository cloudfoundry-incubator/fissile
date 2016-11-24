package builder

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/util"

	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

func TestGenerateRoleImageDockerfile(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		&bytes.Buffer{},
		ioutil.Discard,
		nil,
	)

	releaseVersion := "3.14.15"

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathCache := filepath.Join(releasePath, "bosh-cache")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewDevRelease(releasePath, "", "", releasePathCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "", releaseVersion, "6.28.30", ui)
	assert.NoError(err)

	var dockerfileContents bytes.Buffer
	baseImage := GetBaseImageName(roleImageBuilder.repository, roleImageBuilder.fissileVersion)
	err = roleImageBuilder.generateDockerfile(rolesManifest.Roles[0], baseImage, &dockerfileContents)
	assert.NoError(err)

	dockerfileString := dockerfileContents.String()
	assert.Contains(dockerfileString, "foo-role-base:6.28.30")
	assert.Contains(dockerfileString, "MAINTAINER", "release images should contain maintainer information")
	assert.Contains(
		dockerfileString,
		fmt.Sprintf(`LABEL "role"="%s" "version"="%s"`, rolesManifest.Roles[0].Name, releaseVersion),
		"Expected role label",
	)

	dockerfileContents.Reset()
	err = roleImageBuilder.generateDockerfile(rolesManifest.Roles[0], baseImage, &dockerfileContents)
	assert.NoError(err)
	dockerfileString = dockerfileContents.String()
	assert.Contains(dockerfileString, "MAINTAINER", "dev mode should generate a maintainer layer")
}

func TestGenerateRoleImageRunScript(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		&bytes.Buffer{},
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathCache := filepath.Join(releasePath, "bosh-cache")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewDevRelease(releasePath, "", "", releasePathCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "", "3.14.15", "6.28.30", ui)
	assert.NoError(err)

	runScriptContents, err := roleImageBuilder.generateRunScript(rolesManifest.Roles[0])
	assert.NoError(err)
	assert.Contains(string(runScriptContents), "source /opt/hcf/startup/environ.sh")
	assert.Contains(string(runScriptContents), "source /environ/script/with/absolute/path.sh")
	assert.NotContains(string(runScriptContents), "/opt/hcf/startup/environ/script/with/absolute/path.sh")
	assert.NotContains(string(runScriptContents), "/opt/hcf/startup//environ/script/with/absolute/path.sh")
	assert.Contains(string(runScriptContents), "bash /opt/hcf/startup/myrole.sh")
	assert.Contains(string(runScriptContents), "bash /script/with/absolute/path.sh")
	assert.NotContains(string(runScriptContents), "/opt/hcf/startup/script/with/absolute/path.sh")
	assert.NotContains(string(runScriptContents), "/opt/hcf/startup//script/with/absolute/path.sh")
	assert.Contains(string(runScriptContents), "bash /opt/hcf/startup/post_config_script.sh")
	assert.Contains(string(runScriptContents), "bash /var/vcap/jobs/myrole/pre-start")
	assert.NotContains(string(runScriptContents), "/opt/hcf/startup/var/vcap/jobs/myrole/pre-start")
	assert.NotContains(string(runScriptContents), "/opt/hcf//startup/var/vcap/jobs/myrole/pre-start")
	assert.Contains(string(runScriptContents), "exec dumb-init -- monit -vI")

	runScriptContents, err = roleImageBuilder.generateRunScript(rolesManifest.Roles[1])
	assert.NoError(err)
	assert.NotContains(string(runScriptContents), "monit -vI")
	assert.Contains(string(runScriptContents), "/var/vcap/jobs/tor/bin/run")
}

func TestGenerateRoleImageJobsConfig(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		&bytes.Buffer{},
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathCache := filepath.Join(releasePath, "bosh-cache")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewDevRelease(releasePath, "", "", releasePathCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "", "3.14.15", "6.28.30", ui)
	assert.NoError(err)

	jobsConfigContents, err := roleImageBuilder.generateJobsConfig(rolesManifest.Roles[0])
	assert.NoError(err)
	assert.Contains(string(jobsConfigContents), "/var/vcap/jobs/tor/bin/tor_ctl")
	assert.Contains(string(jobsConfigContents), "/var/vcap/jobs-src/tor/templates/data/properties.sh.erb")
	assert.Contains(string(jobsConfigContents), "/etc/monitrc")
	assert.Contains(string(jobsConfigContents), "/var/vcap/jobs/new_hostname/bin/run")

	jobsConfigContents, err = roleImageBuilder.generateJobsConfig(rolesManifest.Roles[1])
	assert.NoError(err)
	assert.Contains(string(jobsConfigContents), "/var/vcap/jobs/tor/bin/tor_ctl")
	assert.Contains(string(jobsConfigContents), "/var/vcap/jobs-src/tor/templates/data/properties.sh.erb")
	assert.NotContains(string(jobsConfigContents), "/etc/monitrc")
	assert.NotContains(string(jobsConfigContents), "/var/vcap/jobs/new_hostname/bin/run")
}

func TestGenerateRoleImageDockerfileDir(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		&bytes.Buffer{},
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathCache := filepath.Join(releasePath, "bosh-cache")
	releasePathConfigSpec := filepath.Join(releasePath, "config_spec")

	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewDevRelease(releasePath, "", "", releasePathCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "", "3.14.15", "6.28.30", ui)
	assert.NoError(err)

	dockerfileDir, err := roleImageBuilder.CreateDockerfileDir(
		rolesManifest.Roles[0],
		releasePathConfigSpec,
	)
	assert.NoError(err)
	defer os.RemoveAll(dockerfileDir)

	assert.Equal(targetPath, filepath.Dir(dockerfileDir), "Docker file %s not created in directory %s", dockerfileDir, targetPath)

	for _, info := range []struct {
		path  string
		isDir bool
		desc  string
	}{
		{path: ".", isDir: true, desc: "role dir"},
		{path: "Dockerfile", isDir: false, desc: "Dockerfile"},
		{path: "root", isDir: true, desc: "image root"},
		{path: "root/opt/hcf/share/doc/tor/LICENSE", isDir: false, desc: "release license file"},
		{path: "root/opt/hcf/run.sh", isDir: false, desc: "run script"},
		{path: "root/opt/hcf/startup/", isDir: true, desc: "role startup scripts dir"},
		{path: "root/opt/hcf/startup/myrole.sh", isDir: false, desc: "role specific startup script"},
		{path: "root/var/vcap/jobs-src/tor/monit", isDir: false, desc: "job monit file"},
		{path: "root/var/vcap/jobs-src/tor/templates/bin/monit_debugger", isDir: false, desc: "job template file"},
		{path: "root/var/vcap/packages/tor", isDir: false, desc: "package symlink"},
	} {
		path := filepath.ToSlash(filepath.Join(dockerfileDir, info.path))
		assert.NoError(util.ValidatePath(path, info.isDir, info.desc))
	}

	symlinkPath := filepath.Join(dockerfileDir, "root/var/vcap/packages/tor")
	if pathInfo, err := os.Lstat(symlinkPath); assert.NoError(err) {
		assert.Equal(os.ModeSymlink, pathInfo.Mode()&os.ModeSymlink)
		if target, err := os.Readlink(symlinkPath); assert.NoError(err) {
			pkg := getPackage(rolesManifest.Roles, "myrole", "tor", "tor")
			if assert.NotNil(pkg, "Failed to find package") {
				expectedTarget := filepath.Join("..", "packages-src", pkg.Fingerprint)
				assert.Equal(expectedTarget, target)
			}
		}
	}

	// job.MF should not be there
	assert.Error(util.ValidatePath(filepath.ToSlash(filepath.Join(dockerfileDir, "root/var/vcap/jobs-src/tor/job.MF")), false, "job manifest file"))
}

// getPackage is a helper to get a package from a list of roles
func getPackage(roles model.Roles, role, job, pkg string) *model.Package {
	for _, r := range roles {
		if r.Name != role {
			continue
		}
		for _, j := range r.Jobs {
			if j.Name != job {
				continue
			}
			for _, p := range j.Packages {
				if p.Name == pkg {
					return p
				}
			}
		}
	}
	return nil
}

type buildImageCallback func(name string) error

type mockDockerImageBuilder struct {
	callback buildImageCallback
	hasImage bool
}

func (m *mockDockerImageBuilder) BuildImage(dockerDirPath, name string, stdoutProcessor io.WriteCloser) error {
	return m.callback(name)
}

func (m *mockDockerImageBuilder) HasImage(imageName string) (bool, error) {
	return m.hasImage, nil
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
		&bytes.Buffer{},
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathCache := filepath.Join(releasePath, "bosh-cache")

	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)

	release, err := model.NewDevRelease(releasePath, "", "", releasePathCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	roleImageBuilder, err := NewRoleImageBuilder(
		"test-repository",
		compiledPackagesDir,
		targetPath,
		"",
		"3.14.15",
		"6.28.30",
		ui,
	)
	assert.NoError(err)

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
		"",
		false,
		false,
		2,
	)
	assert.NoError(err)

	err = os.RemoveAll(targetPath)
	assert.NoError(err, "Failed to remove target")

	targetPath, err = ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)
	roleImageBuilder.targetPath = targetPath

	// Should not allow invalid worker counts
	err = roleImageBuilder.BuildRoleImages(
		rolesManifest.Roles,
		"test-repository",
		"",
		false,
		false,
		0,
	)
	assert.Error(err, "Invalid worker count should result in an error")
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
		"",
		false,
		false,
		1,
	)
	assert.Contains(err.Error(), "Deliberate failure", "Returned error should be from first job failing")
	assert.False(hasRunSecondJob, "Second job should not have run")

	// Check that we do not attempt to rebuild images
	mockBuilder.hasImage = true
	var buildersRan []string
	mockBuilder.callback = func(name string) error {
		buildersRan = append(buildersRan, name)
		return nil
	}
	err = roleImageBuilder.BuildRoleImages(
		rolesManifest.Roles,
		"test-repository",
		"",
		false,
		false,
		len(rolesManifest.Roles),
	)
	assert.NoError(err)
	assert.Empty(buildersRan, "should not have ran any builders")

	// Check that we write timestamps to the metrics file
	file, err := ioutil.TempFile("", "metrics")
	assert.NoError(err)

	metrics := file.Name()
	defer os.Remove(metrics)
	roleImageBuilder.metricsPath = metrics

	err = os.RemoveAll(targetPath)
	assert.NoError(err, "Failed to remove target")

	targetPath, err = ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)
	roleImageBuilder.targetPath = targetPath

	mockBuilder.hasImage = false
	mockBuilder.callback = func(name string) error {
		return nil
	}
	err = roleImageBuilder.BuildRoleImages(
		rolesManifest.Roles,
		"test-repository",
		"",
		false,
		false,
		1,
	)
	assert.NoError(err)

	expected := `.*,fissile,create-role-images::test-repository-myrole:[a-z0-9]{40},start
.*,fissile,create-role-images::test-repository-myrole:[a-z0-9]{40},done
.*,fissile,create-role-images::test-repository-foorole:[a-z0-9]{40},start
.*,fissile,create-role-images::test-repository-foorole:[a-z0-9]{40},done`

	contents, err := ioutil.ReadFile(metrics)
	assert.NoError(err)
	assert.Regexp(regexp.MustCompile(expected), string(contents))
}
