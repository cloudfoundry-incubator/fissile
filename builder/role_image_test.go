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
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, release)
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14")

	dockerfileContents, err := roleImageBuilder.generateDockerfile(rolesManifest.Roles[0])
	assert.Nil(err)

	assert.Contains(string(dockerfileContents), "foo:role-base")
	assert.Contains(string(dockerfileContents), `"release-version"="0.3.5"`)
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
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, release)
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14")

	runScriptContents, err := roleImageBuilder.generateRunScript(rolesManifest.Roles[0])
	assert.Nil(err)

	assert.Contains(string(runScriptContents), "/var/vcap/jobs-src/tor/templates/data/properties.sh.erb")
	assert.Contains(string(runScriptContents), "/opt/hcf/monitrc.erb")
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
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, release)
	assert.Nil(err)

	roleImageBuilder := NewRoleImageBuilder("foo", compiledPackagesDir, targetPath, "http://127.0.0.1:8500", "hcf", "3.14")

	dockerfileDir, err := roleImageBuilder.CreateDockerfileDir(rolesManifest.Roles[0])
	assert.Nil(err)

	assert.Equal(filepath.Join(targetPath, "myrole"), dockerfileDir)

	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole"), true, "role dir"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "run.sh"), false, "run script"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "Dockerfile"), false, "Dockerfile"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "packages", "tor"), true, "package dir"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "packages", "tor", "bar"), false, "compilation artifact"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "jobs", "tor", "monit"), false, "job monit file"))
	assert.Nil(util.ValidatePath(filepath.Join(targetPath, "myrole", "jobs", "tor", "templates", "bin", "monit_debugger"), false, "job template file"))

	// job.MF should not be there
	assert.NotNil(util.ValidatePath(filepath.Join(targetPath, "myrole", "jobs", "tor", "job.MF"), false, "job manifest file"))
}
