package builder

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/SUSE/fissile/model"

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

	torOpinionsDir := filepath.Join(workDir, "../test-assets/tor-opinions")
	lightOpinionsPath := filepath.Join(torOpinionsDir, "opinions.yml")
	darkOpinionsPath := filepath.Join(torOpinionsDir, "dark-opinions.yml")
	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, lightOpinionsPath, darkOpinionsPath, "", "6.28.30", ui)
	assert.NoError(err)

	var dockerfileContents bytes.Buffer
	baseImage := roleImageBuilder.repository
	err = roleImageBuilder.generateDockerfile(rolesManifest.Roles[0], baseImage, &dockerfileContents)
	assert.NoError(err)

	dockerfileString := dockerfileContents.String()
	assert.Contains(dockerfileString, "foo")
	assert.Contains(dockerfileString, "MAINTAINER", "release images should contain maintainer information")
	assert.Contains(
		dockerfileString,
		fmt.Sprintf(`LABEL "role"="%s"`, rolesManifest.Roles[0].Name),
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
	torOpinionsDir := filepath.Join(workDir, "../test-assets/tor-opinions")
	lightOpinionsPath := filepath.Join(torOpinionsDir, "opinions.yml")
	darkOpinionsPath := filepath.Join(torOpinionsDir, "dark-opinions.yml")

	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, lightOpinionsPath, darkOpinionsPath, "", "6.28.30", ui)
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
	assert.Contains(string(runScriptContents), "monit -vI &")

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

	torOpinionsDir := filepath.Join(workDir, "../test-assets/tor-opinions")
	lightOpinionsPath := filepath.Join(torOpinionsDir, "opinions.yml")
	darkOpinionsPath := filepath.Join(torOpinionsDir, "dark-opinions.yml")
	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, lightOpinionsPath, darkOpinionsPath, "", "6.28.30", ui)
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

	torOpinionsDir := filepath.Join(workDir, "../test-assets/tor-opinions")
	lightOpinionsPath := filepath.Join(torOpinionsDir, "opinions.yml")
	darkOpinionsPath := filepath.Join(torOpinionsDir, "dark-opinions.yml")

	roleImageBuilder, err := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, lightOpinionsPath, darkOpinionsPath, "", "6.28.30", ui)
	assert.NoError(err)

	torPkg := getPackage(rolesManifest.Roles, "myrole", "tor", "tor")

	const TypeMissing byte = tar.TypeCont // flag to indicate an expected missing file
	expected := map[string]struct {
		desc     string
		typeflag byte // default to TypeRegA
		keep     bool // Hold for extra examination after
	}{
		"Dockerfile":                                              {desc: "Dockerfile"},
		"root/opt/hcf/share/doc/tor/LICENSE":                      {desc: "release license file"},
		"root/opt/hcf/run.sh":                                     {desc: "run script"},
		"root/opt/hcf/startup/myrole.sh":                          {desc: "role specific startup script"},
		"root/var/vcap/jobs-src/tor/monit":                        {desc: "job monit file"},
		"root/var/vcap/jobs-src/tor/templates/bin/monit_debugger": {desc: "job template file"},
		"root/var/vcap/jobs-src/tor/config_spec.json":             {desc: "tor config spec", keep: true},
		"root/var/vcap/jobs-src/new_hostname/config_spec.json":    {desc: "new_hostname config spec", keep: true},
		"root/var/vcap/packages/tor":                              {desc: "package symlink", typeflag: tar.TypeSymlink, keep: true},
		"root/var/vcap/jobs-src/tor/job.MF":                       {desc: "job manifest file", typeflag: TypeMissing},
	}
	actual := make(map[string][]byte)

	populator := roleImageBuilder.NewDockerPopulator(rolesManifest.Roles[0], releasePathConfigSpec)

	pipeR, pipeW, err := os.Pipe()
	assert.NoError(err, "Failed to create a pipe")

	tarWriter := tar.NewWriter(pipeW)
	tarReader := tar.NewReader(pipeR)
	var asyncError error
	latch := make(chan struct{})
	go func() {
		defer close(latch)
		defer tarWriter.Close()
		asyncError = populator(tarWriter)
	}()

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if !assert.NoError(err, "Error reading tar file") {
			break
		}
		if info, ok := expected[header.Name]; ok {
			delete(expected, header.Name)
			if info.typeflag == tar.TypeRegA {
				info.typeflag = tar.TypeReg
			}
			if header.Typeflag == tar.TypeRegA {
				header.Typeflag = tar.TypeReg
			}
			if info.typeflag == TypeMissing {
				assert.Fail("File %s should not exist", header.Name)
				continue
			}
			assert.Equal(info.typeflag, header.Typeflag, "Unexpected type for item %s", header.Name)
			if info.keep {
				if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
					actual[header.Name] = []byte(header.Linkname)
				} else {
					buf := &bytes.Buffer{}
					_, err = io.Copy(buf, tarReader)
					assert.NoError(err, "Error reading contents of %s", header.Name)
					actual[header.Name] = buf.Bytes()
				}
			}
		}
	}
	// Synchronize with the gofunc to make sure it's done
	<-latch
	for name, info := range expected {
		assert.Equal(TypeMissing, info.typeflag, "File %s was not found", name)
	}

	if assert.Contains(actual, "root/var/vcap/packages/tor", "tor package missing") {
		expectedTarget := filepath.Join("..", "packages-src", torPkg.Fingerprint)
		assert.Equal(string(actual["root/var/vcap/packages/tor"]), expectedTarget)
	}

	// And verify the config specs are as expected
	if assert.Contains(actual, "root/var/vcap/jobs-src/new_hostname/config_spec.json") {
		buf := actual["root/var/vcap/jobs-src/new_hostname/config_spec.json"]
		var result map[string]interface{}
		err = json.Unmarshal(buf, &result)
		if !assert.NoError(err, "Error unmarshalling output") {
			return
		}
		assert.Empty(result["properties"].(map[string]interface{}))
	}

	if assert.Contains(actual, "root/var/vcap/jobs-src/tor/config_spec.json") {
		buf := actual["root/var/vcap/jobs-src/tor/config_spec.json"]

		expectedString := `{
			"job": {
				"name": "myrole",
				"templates": [
					{"name":"new_hostname"},
					{"name":"tor"}
				]
			},
			"networks":{
				"default":{}
			},
			"parameters":{},
			"properties": {
				"tor": {
					"hashed_control_password":null,
					"hostname":"localhost",
					"private_key": null,
					"client_keys":null
				}
			}
		}`
		assert.JSONEq(expectedString, string(buf))
	}
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
	tarBytes map[string]*bytes.Buffer
	mutex    sync.Mutex
}

func (m *mockDockerImageBuilder) BuildImage(dockerDirPath, name string, stdoutProcessor io.WriteCloser) error {
	return m.callback(name)
}

func (m *mockDockerImageBuilder) BuildImageFromCallback(name string, stdoutProcessor io.Writer, populator func(*tar.Writer) error) error {
	if err := m.callback(name); err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	(func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		if m.tarBytes == nil {
			m.tarBytes = make(map[string]*bytes.Buffer)
		}
		m.tarBytes[name] = buf
	})()
	tarWriter := tar.NewWriter(buf)
	return populator(tarWriter)
}

func (m *mockDockerImageBuilder) HasImage(imageName string) (bool, error) {
	return m.hasImage, nil
}

func TestBuildRoleImages(t *testing.T) {

	origNewDockerImageBuilder := newDockerImageBuilder
	defer func() {
		newDockerImageBuilder = origNewDockerImageBuilder
	}()

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
	torOpinionsDir := filepath.Join(workDir, "../test-assets/tor-opinions")
	lightOpinionsPath := filepath.Join(torOpinionsDir, "opinions.yml")
	darkOpinionsPath := filepath.Join(torOpinionsDir, "dark-opinions.yml")

	roleImageBuilder, err := NewRoleImageBuilder(
		"test-repository",
		compiledPackagesDir,
		targetPath,
		lightOpinionsPath,
		darkOpinionsPath,
		"",
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
		"test-registry.com:9000",
		"test-organization",
		"test-repository",
		"",
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
		"test-registry.com:9000",
		"test-organization",
		"test-repository",
		"",
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
		"test-registry.com:9000",
		"test-organization",
		"test-repository",
		"",
		"",
		false,
		false,
		1,
	)
	if assert.Error(err) {
		assert.Contains(err.Error(), "Deliberate failure", "Returned error should be from first job failing")
	}
	assert.False(hasRunSecondJob, "Second job should not have run")

	// Check that we do not attempt to rebuild images
	mockBuilder.hasImage = true
	var buildersRan []string
	mutex := sync.Mutex{}
	mockBuilder.callback = func(name string) error {
		mutex.Lock()
		defer mutex.Unlock()
		buildersRan = append(buildersRan, name)
		return nil
	}
	err = roleImageBuilder.BuildRoleImages(
		rolesManifest.Roles,
		"test-registry.com:9000",
		"test-organization",
		"test-repository",
		"",
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
		"test-registry.com:9000",
		"test-organization",
		"test-repository",
		"",
		"",
		false,
		false,
		1,
	)
	assert.NoError(err)

	expected := `.*,fissile,create-role-images::test-registry.com:9000/test-organization/test-repository-myrole:[a-z0-9]{40},start
.*,fissile,create-role-images::test-registry.com:9000/test-organization/test-repository-myrole:[a-z0-9]{40},done
.*,fissile,create-role-images::test-registry.com:9000/test-organization/test-repository-foorole:[a-z0-9]{40},start
.*,fissile,create-role-images::test-registry.com:9000/test-organization/test-repository-foorole:[a-z0-9]{40},done`

	contents, err := ioutil.ReadFile(metrics)
	assert.NoError(err)
	assert.Regexp(regexp.MustCompile(expected), string(contents))
}

func TestGetRoleDevImageName(t *testing.T) {
	assert := assert.New(t)

	var role model.Role

	role.Name = "foorole"

	reg := "test-registry:9000"
	org := "test-org"
	repo := "test-repository"
	version := "a886ed76c6d6e5a96ad5c37fb208368a430a29d770f1d149a78e1e6e8091eb12"

	// Test with repository only
	expected := "test-repository-foorole:a886ed76c6d6e5a96ad5c37fb208368a430a29d770f1d149a78e1e6e8091eb12"
	imageName := GetRoleDevImageName("", "", repo, &role, version)
	assert.Equal(expected, imageName)

	// Test with org and repository
	expected = "test-org/test-repository-foorole:a886ed76c6d6e5a96ad5c37fb208368a430a29d770f1d149a78e1e6e8091eb12"
	imageName = GetRoleDevImageName("", org, repo, &role, version)
	assert.Equal(expected, imageName)

	// Test with registry and repository
	expected = "test-registry:9000/test-repository-foorole:a886ed76c6d6e5a96ad5c37fb208368a430a29d770f1d149a78e1e6e8091eb12"
	imageName = GetRoleDevImageName(reg, "", repo, &role, version)
	assert.Equal(expected, imageName)

	// Test with all three
	expected = "test-registry:9000/test-org/test-repository-foorole:a886ed76c6d6e5a96ad5c37fb208368a430a29d770f1d149a78e1e6e8091eb12"
	imageName = GetRoleDevImageName(reg, org, repo, &role, version)
	assert.Equal(expected, imageName)
}
