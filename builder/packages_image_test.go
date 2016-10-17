package builder

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/util"
	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

func TestCreatePackagesDockerBuildDir(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		os.Stdin,
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

	roleImageBuilder, err := NewPackagesImageBuilder("foo", compiledPackagesDir, targetPath, "3.14.15", ui)
	assert.NoError(err)

	dockerfileDir, err := roleImageBuilder.CreatePackagesDockerBuildDir(
		rolesManifest,
		filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml"),
		filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml"),
	)
	assert.NoError(err)
	defer os.RemoveAll(dockerfileDir)

	assert.Equal(targetPath, filepath.Dir(dockerfileDir), "Docker file %s not created in directory %s", dockerfileDir, targetPath)

	pkg := getPackage(rolesManifest.Roles, "myrole", "tor", "tor")
	if !assert.NotNil(pkg) {
		return
	}

	for _, info := range []struct {
		path  string
		isDir bool
		desc  string
	}{
		{path: ".", isDir: true, desc: "role dir"},
		{path: "Dockerfile", desc: "Dockerfile"},
		{path: "root", isDir: true, desc: "image root"},
		{path: filepath.Join("root/var/vcap/packages-src", pkg.Fingerprint), isDir: true, desc: "package dir"},
		{path: filepath.Join("root/var/vcap/packages-src", pkg.Fingerprint, "bar"), desc: "compilation artifact"},
		{path: "root/opt/hcf/specs/myrole/tor.json", desc: "job spec"},
	} {
		path := filepath.ToSlash(filepath.Join(dockerfileDir, info.path))
		assert.NoError(util.ValidatePath(path, info.isDir, info.desc))
	}

	// jobs-src should not be there
	assert.Error(util.ValidatePath(filepath.ToSlash(filepath.Join(dockerfileDir, "root/var/vcap/jobs-src")), true, "job directory"))
}
