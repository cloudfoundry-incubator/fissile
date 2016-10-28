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

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

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

	packagesImageBuilder, err := NewPackagesImageBuilder("foo", compiledPackagesDir, targetPath, "3.14.15", ui)
	assert.NoError(err)

	dockerfile := bytes.Buffer{}

	err = packagesImageBuilder.generateDockerfile("scratch:latest", nil, &dockerfile)
	assert.NoError(err)

	lines := getDockerfileLines(dockerfile.String())
	assert.Equal([]string{
		"FROM scratch:latest",
		"ADD specs /opt/hcf/specs",
		"ADD packages-src /var/vcap/packages-src/",
	}, lines, "Unexpected dockerfile contents found")
}

func TestCreatePackagesDockerStream(t *testing.T) {
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

	packagesImageBuilder, err := NewPackagesImageBuilder("foo", compiledPackagesDir, targetPath, "3.14.15", ui)
	assert.NoError(err)

	tarStream, errors, err := packagesImageBuilder.CreatePackagesDockerStream(
		rolesManifest,
		filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml"),
		filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml"),
		false,
	)
	assert.NoError(err)
	defer tarStream.Close()

	pkg := getPackage(rolesManifest.Roles, "myrole", "tor", "tor")
	if !assert.NotNil(pkg) {
		return
	}

	// Get the docker id for the image we'll be building from...
	dockerManager, err := docker.NewImageManager()
	assert.NoError(err)
	baseImage, err := dockerManager.FindImage(baseImageOverride)
	assert.NoError(err)

	testFunctions := map[string]func(string){
		"Dockerfile": func(contents string) {
			var i int
			var line string
			testers := []func(){
				func() { assert.Equal(fmt.Sprintf("FROM %s", baseImage.ID), line, "line 1 should start with FROM") },
				func() { assert.Equal("ADD specs /opt/hcf/specs", line, "line 2 mismatch") },
				func() { assert.Equal("ADD packages-src /var/vcap/packages-src/", line, "line 3 mismatch") },
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
		"specs/foorole/tor.json": func(contents string) {
			expected := `{
				"job": {
					"name": "foorole",
					"templates": [{"name": "tor"}]
				},
				"networks": {
					"default": {}
				},
				"parameters": {},
				"properties": {
					"tor": {
						"client_keys": null,
						"hashed_control_password": null,
						"hostname": "localhost",
						"private_key": null
					}
				}
			}`
			assert.JSONEq(expected, string(contents))
		},
		"packages-src/b9973278a447dfb5e8e67661deaa5fe7001ad742/bar": func(contents string) {
			assert.Empty(contents)
		},
	}
	tarReader := tar.NewReader(tarStream)
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

	assert.NoError(<-errors)
}
