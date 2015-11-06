package compilator

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"
	"github.com/hpcloud/fissile/util"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	dockerImageEnvVar      = "FISSILE_TEST_DOCKER_IMAGE"
	defaultDockerTestImage = "ubuntu:14.04"
)

var dockerImageName string

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)

	dockerImageName = os.Getenv(dockerImageEnvVar)
	if dockerImageName == "" {
		dockerImageName = defaultDockerTestImage
	}

	retCode := m.Run()

	os.Exit(retCode)
}

func TestCompilationEmpty(t *testing.T) {
	assert := assert.New(t)

	c, err := NewCompilator(nil, "", "", "", "")
	assert.Nil(err)

	waitCh := make(chan struct{})
	go func() {
		err := c.Compile(1, genTestCase())
		close(waitCh)
		assert.Nil(err)
	}()

	<-waitCh
}

func TestCompilationBasic(t *testing.T) {
	saveCompilePackage := compilePackageHarness
	defer func() {
		compilePackageHarness = saveCompilePackage
	}()

	compileChan := make(chan string)
	compilePackageHarness = func(c *Compilator, pkg *model.Package) error {
		compileChan <- pkg.Name
		return nil
	}

	assert := assert.New(t)

	c, err := NewCompilator(nil, "", "", "", "")
	assert.Nil(err)

	release := genTestCase("ruby-2.5", "consul>go-1.4", "go-1.4")

	waitCh := make(chan struct{})
	go func() {
		c.Compile(1, release)
		close(waitCh)
	}()

	assert.Equal(<-compileChan, "ruby-2.5")
	assert.Equal(<-compileChan, "go-1.4")
	assert.Equal(<-compileChan, "consul")
	<-waitCh
}

func TestCompilationSkipCompiled(t *testing.T) {
	saveCompilePackage := compilePackageHarness
	saveIsPackageCompiled := isPackageCompiledHarness
	defer func() {
		compilePackageHarness = saveCompilePackage
		isPackageCompiledHarness = saveIsPackageCompiled
	}()

	compileChan := make(chan string)
	compilePackageHarness = func(c *Compilator, pkg *model.Package) error {
		compileChan <- pkg.Name
		return nil
	}

	isPackageCompiledHarness = func(c *Compilator, pkg *model.Package) (bool, error) {
		return pkg.Name == "ruby-2.5", nil
	}

	assert := assert.New(t)

	c, err := NewCompilator(nil, "", "", "", "")
	assert.Nil(err)

	release := genTestCase("ruby-2.5", "consul>go-1.4", "go-1.4")

	waitCh := make(chan struct{})
	go func() {
		c.Compile(1, release)
		close(waitCh)
	}()

	assert.Equal(<-compileChan, "go-1.4")
	assert.Equal(<-compileChan, "consul")
	<-waitCh
}

func TestGetPackageStatusCompiled(t *testing.T) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := model.NewRelease(ntpReleasePath)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test", compilation.FakeBase, "3.14.15")
	assert.Nil(err)

	compilator.initPackageMaps(release)

	compiledPackagePath := filepath.Join(compilationWorkDir, release.Packages[0].Name, release.Packages[0].Fingerprint, "compiled")
	err = os.MkdirAll(compiledPackagePath, 0755)
	assert.Nil(err)

	err = ioutil.WriteFile(filepath.Join(compiledPackagePath, "foo"), []byte{}, 0700)
	assert.Nil(err)

	status, err := compilator.isPackageCompiled(release.Packages[0])

	assert.Nil(err)
	assert.True(status)
}

func TestGetPackageStatusNone(t *testing.T) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := model.NewRelease(ntpReleasePath)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test", compilation.FakeBase, "3.14.15")
	assert.Nil(err)

	compilator.initPackageMaps(release)

	status, err := compilator.isPackageCompiled(release.Packages[0])

	assert.Nil(err)
	assert.False(status)
}

func TestPackageFolderStructure(t *testing.T) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := model.NewRelease(ntpReleasePath)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test", compilation.FakeBase, "3.14.15")
	assert.Nil(err)

	err = compilator.createCompilationDirStructure(release.Packages[0])
	assert.Nil(err)

	exists, err := validatePath(compilator.getDependenciesPackageDir(release.Packages[0]), true, "")
	assert.Nil(err)
	assert.True(exists)

	exists, err = validatePath(compilator.getSourcePackageDir(release.Packages[0]), true, "")
	assert.Nil(err)
	assert.True(exists)
}

func TestPackageDependenciesPreparation(t *testing.T) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease-0.3.5")
	release, err := model.NewRelease(ntpReleasePath)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test", compilation.FakeBase, "3.14.15")
	assert.Nil(err)

	pkg, err := release.LookupPackage("tor")
	assert.Nil(err)
	err = compilator.createCompilationDirStructure(pkg)
	assert.Nil(err)
	err = os.MkdirAll(compilator.getPackageCompiledDir(pkg.Dependencies[0]), 0755)
	assert.Nil(err)

	dummyCompiledFile := filepath.Join(compilator.getPackageCompiledDir(pkg.Dependencies[0]), "foo")
	file, err := os.Create(dummyCompiledFile)
	assert.Nil(err)
	file.Close()

	err = compilator.copyDependencies(pkg)
	assert.Nil(err)

	expectedDummyFileLocation := filepath.Join(compilator.getDependenciesPackageDir(pkg), pkg.Dependencies[0].Name, "foo")
	exists, err := validatePath(expectedDummyFileLocation, false, "")
	assert.Nil(err)
	assert.True(exists, expectedDummyFileLocation)
}

func TestCompilePackage(t *testing.T) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release-2")
	release, err := model.NewRelease(ntpReleasePath)
	assert.Nil(err)

	testRepository := fmt.Sprintf("fissile-test-%s", uuid.New())

	comp, err := NewCompilator(dockerManager, compilationWorkDir, testRepository, compilation.FakeBase, "3.14.15")
	assert.Nil(err)

	imageName := comp.BaseImageName()

	_, err = comp.CreateCompilationBase(dockerImageName)
	defer func() {
		err = dockerManager.RemoveImage(imageName)
		assert.Nil(err)
	}()
	assert.Nil(err)

	err = comp.compilePackage(release.Packages[0])
	assert.Nil(err)
}

func TestCreateDepBuckets(t *testing.T) {
	t.Parallel()

	packages := []*model.Package{
		{
			Name: "consul",
			Dependencies: []*model.Package{
				{Name: "go-1.4"},
			},
		},
		{
			Name:         "go-1.4",
			Dependencies: nil,
		},
		{
			Name: "cloud_controller_go",
			Dependencies: []*model.Package{
				{Name: "go-1.4"},
				{Name: "ruby-2.5"},
			},
		},
		{
			Name:         "ruby-2.5",
			Dependencies: nil,
		},
	}

	buckets := createDepBuckets(packages)
	assert.Equal(t, len(buckets), 3)
	assert.Equal(t, len(buckets[0]), 2)
	assert.Equal(t, buckets[0][0].Name, "ruby-2.5") // Ruby should be first
	assert.Equal(t, buckets[0][1].Name, "go-1.4")
	assert.Equal(t, len(buckets[1]), 1)
	assert.Equal(t, buckets[1][0].Name, "consul")
	assert.Equal(t, len(buckets[2]), 1)
	assert.Equal(t, buckets[2][0].Name, "cloud_controller_go")
}

func TestRemoveCompiledPackages(t *testing.T) {
	saveIsPackageCompiled := isPackageCompiledHarness
	defer func() {
		isPackageCompiledHarness = saveIsPackageCompiled
	}()

	isPackageCompiledHarness = func(c *Compilator, pkg *model.Package) (bool, error) {
		return pkg.Name == "ruby-2.5", nil
	}

	assert := assert.New(t)

	c, err := NewCompilator(nil, "", "", "", "")
	assert.Nil(err)

	release := genTestCase("ruby-2.5", "consul>go-1.4", "go-1.4")

	c.initPackageMaps(release)
	packages, err := c.removeCompiledPackages(release.Packages)
	assert.Nil(err)

	assert.Equal(2, len(packages))
	assert.Equal(packages[0].Name, "consul")
	assert.Equal(packages[1].Name, "go-1.4")
}

func genTestCase(args ...string) *model.Release {
	var packages []*model.Package

	for _, pkgDef := range args {
		splits := strings.Split(pkgDef, ">")
		pkgName := splits[0]

		var deps []*model.Package
		if len(splits) == 2 {
			pkgDeps := strings.Split(splits[1], ",")

			for _, dep := range pkgDeps {
				deps = append(deps, &model.Package{Name: dep})
			}
		}

		packages = append(packages, &model.Package{Name: pkgName, Dependencies: deps})
	}

	return &model.Release{Packages: packages}
}
