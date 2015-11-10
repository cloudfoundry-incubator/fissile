package builder

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/util"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRoleImageDockerfile(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.Nil(err)

	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14.15", "6.28.30")

	dockerfileContents, err := roleImageBuilder.generateDockerfile(rolesManifest.Roles[0])
	assert.Nil(err)

	dockerfileString := string(dockerfileContents)
	assert.Contains(dockerfileString, "foo-role-base:6.28.30")
	assert.Contains(dockerfileString, "ADD LICENSE.md  /opt/hcf/share/doc")
}

func TestGenerateRoleImageRunScript(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.Nil(err)

	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14.15", "6.28.30")

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

	workDir, err := os.Getwd()
	assert.Nil(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.Nil(err)

	release, err := model.NewRelease(releasePath)
	assert.Nil(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14.15", "6.28.30")

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
