package builder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/SUSE/fissile/docker"
	"github.com/SUSE/fissile/model"

	"github.com/SUSE/termui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
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

// Given the contents of a Dockerfile, return each non-comment line in an array
func getDockerfileLines(text string) []string {
	var result []string
	for _, line := range strings.Split(text, "\n") {
		line := strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			result = append(result, line)
		}
	}
	return result
}

func TestGenerateDockerfile(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		&bytes.Buffer{},
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.NoError(err)

	compiledPackagesDir := filepath.Join(workDir, "../test-assets/tor-boshrelease-fake-compiled")
	targetPath, err := ioutil.TempDir("", "fissile-test")
	assert.NoError(err)
	defer os.RemoveAll(targetPath)

	packagesImageBuilder, err := NewPackagesImageBuilder("foo", dockerImageName, "", compiledPackagesDir, targetPath, "3.14.15", ui)
	assert.NoError(err)

	dockerfile := bytes.Buffer{}

	err = packagesImageBuilder.generateDockerfile("scratch:latest", nil, &dockerfile)
	assert.NoError(err)

	lines := getDockerfileLines(dockerfile.String())
	assert.Equal([]string{
		"FROM scratch:latest",
		"ADD packages-src /var/vcap/packages-src/",
		"LABEL version.generator.fissile=3.14.15",
	}, lines, "Unexpected dockerfile contents found")
}

func TestNewDockerPopulator(t *testing.T) {
	assert := assert.New(t)

	ui := termui.New(
		&bytes.Buffer{},
		ioutil.Discard,
		nil,
	)

	workDir, err := os.Getwd()
	assert.NoError(err)

	baseImageOverride = defaultDockerTestImage
	defer func() { baseImageOverride = "" }()

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

	packagesImageBuilder, err := NewPackagesImageBuilder("foo", dockerImageName, "", compiledPackagesDir, targetPath, "3.14.15", ui)
	assert.NoError(err)

	tarFile := &bytes.Buffer{}

	tarPopulator := packagesImageBuilder.NewDockerPopulator(rolesManifest.Roles, false)
	tarWriter := tar.NewWriter(tarFile)
	assert.NoError(tarPopulator(tarWriter))
	assert.NoError(tarWriter.Close())

	pkg := getPackage(rolesManifest.Roles, "myrole", "tor", "tor")
	if !assert.NotNil(pkg) {
		return
	}

	// Get the docker id for the image we'll be building from...
	dockerManager, err := docker.NewImageManager()
	assert.NoError(err)
	baseImage, err := dockerManager.FindImage(baseImageOverride)
	assert.NoError(err)

	// From test-assets/tor-boshrelease/dev_releases/tor/tor-0.3.5+dev.3.yml
	const torFingerprint = "59523b1cc4042dff1217ab5b79ff885cdd2de032"

	testFunctions := map[string]func(string){
		"Dockerfile": func(contents string) {
			var i int
			var line string
			testers := []func(){
				func() { assert.Equal(fmt.Sprintf("FROM %s", baseImage.ID), line, "line 1 should start with FROM") },
				func() {
					assert.Equal("ADD packages-src /var/vcap/packages-src/", line, "line 3 mismatch (ADD, package src location)")
				},
				func() {
					assert.Equal("LABEL version.generator.fissile=3.14.15", line, "line 4 mismatch (LABEL, generator version)")
				},
				func() {
					expected := []string{
						"LABEL",
						fmt.Sprintf(`"fingerprint.%s"="libevent"`, getPackage(rolesManifest.Roles, "myrole", "tor", "libevent").Fingerprint),
						fmt.Sprintf(`"fingerprint.%s"="tor"`, getPackage(rolesManifest.Roles, "myrole", "tor", "tor").Fingerprint),
					}
					actual := strings.Fields(line)
					sort.Strings(expected[1:])
					sort.Strings(actual[1:])
					assert.Equal(expected, actual, "line 4 has unexpected fields")
				},
			}
			for i, line = range getDockerfileLines(contents) {
				if assert.True(i < len(testers), "Extra line #%d: %s", i+1, line) {
					testers[i]()
				}
			}
			assert.Equal(len(testers), len(getDockerfileLines(contents)), "Not enough lines")
		},
		"packages-src/" + torFingerprint + "/bar": func(contents string) {
			assert.Empty(contents)
		},
	}

	tarReader := tar.NewReader(tarFile)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if !assert.NoError(err) {
			break
		}
		if tester, ok := testFunctions[header.Name]; ok {
			actual, err := ioutil.ReadAll(tarReader)
			assert.NoError(err)
			tester(string(actual))
			delete(testFunctions, header.Name)
		}
	}
	assert.Empty(testFunctions, "Missing files in tar stream")
}

func setHash(hash map[string]interface{}, value interface{}, keys ...string) {
	var child map[interface{}]interface{}
	for i, k := range keys {
		if i >= len(keys)-1 {
			// don't get the last one
			break
		}
		if i == 0 {
			child = hash[k].(map[interface{}]interface{})
		} else {
			child = child[k].(map[interface{}]interface{})
		}
	}
	child[keys[len(keys)-1]] = value
}

func TestGetRolePackageImageName(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathCache := filepath.Join(releasePath, "bosh-cache")

	release, err := model.NewDevRelease(releasePath, "", "", releasePathCache)
	assert.NoError(t, err)

	roleManifestDir := filepath.Join(workDir, "../test-assets/role-manifests/")
	roleManifestPath := filepath.Join(roleManifestDir, "tor-good.yml")
	roleManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(t, err)

	t.Run("FissileVersionShouldBeRelevant", func(t *testing.T) {
		builder := PackagesImageBuilder{
			repository:      "test",
			fissileVersion:  "0.1.2",
			stemcellImageID: "stemcell:latest",
		}

		oldImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		builder.fissileVersion += ".4.5.6"
		newImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		assert.NotEqual(t, oldImageName, newImageName, "Changing fissile version should change package layer hash")
	})

	t.Run("StemcellImageIDShouldBeRelevant", func(t *testing.T) {
		builder := PackagesImageBuilder{
			repository:      "test",
			fissileVersion:  "0.1.2",
			stemcellImageID: "stemcell:latest",
		}

		oldImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		builder.stemcellImageID = "stemcell:newer"
		newImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		assert.NotEqual(t, oldImageName, newImageName, "Changing stemcell image ID should change package layer hash")
	})

	t.Run("RepositoryShouldBeRelevant", func(t *testing.T) {
		builder := PackagesImageBuilder{
			repository:      "test",
			fissileVersion:  "0.1.2",
			stemcellImageID: "stemcell:latest",
		}
		oldImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		builder.repository = "repository"
		newImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		assert.NotEqual(t, oldImageName, newImageName, "Changing repository should change package layer hash")
	})

	t.Run("TemplatesShouldBeIrrelevant", func(t *testing.T) {
		builder := PackagesImageBuilder{
			repository:      "test",
			fissileVersion:  "0.1.2",
			stemcellImageID: "stemcell:latest",
		}

		oldImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		yamlRaw, err := ioutil.ReadFile(roleManifestPath)
		require.NoError(t, err, "Error reading role manifest")

		var yamlContents map[string]interface{}
		err = yaml.Unmarshal(yamlRaw, &yamlContents)
		require.NoError(t, err)
		setHash(yamlContents, "((#FOO))((/FOO))((BAR))", "configuration", "templates", "properties.tor.hostname")
		yamlBytes, err := yaml.Marshal(yamlContents)
		require.NoError(t, err, "Failed to marshal edited YAML")

		tempManifestFile, err := ioutil.TempFile(roleManifestDir, "fissile-test-role-manifest-")
		require.NoError(t, err, "Error creating temporary file")
		defer os.Remove(tempManifestFile.Name())
		assert.NoError(t, tempManifestFile.Close(), "Error closing temporary file")
		assert.NoError(t, ioutil.WriteFile(tempManifestFile.Name(), yamlBytes, 0644), "Error writing modified role manifest")
		modifiedRoleManifest, err := model.LoadRoleManifest(tempManifestFile.Name(), []*model.Release{release})
		assert.NoError(t, err, "Error loading modified role manifest")

		newImageName, err := builder.GetRolePackageImageName(modifiedRoleManifest, modifiedRoleManifest.Roles)
		assert.NoError(t, err)
		assert.Equal(t, oldImageName, newImageName, "Changing templates should not change image hash")
	})

	t.Run("RolesShouldBeRelevant", func(t *testing.T) {
		builder := PackagesImageBuilder{
			repository:      "test",
			fissileVersion:  "0.1.2",
			stemcellImageID: "stemcell:latest",
		}
		oldImageName, err := builder.GetRolePackageImageName(roleManifest, nil)
		assert.NoError(t, err)

		newImageName, err := builder.GetRolePackageImageName(roleManifest, roleManifest.Roles)
		assert.NoError(t, err)

		assert.NotEqual(t, oldImageName, newImageName, "Changing roles should change package layer hash")
	})
}
