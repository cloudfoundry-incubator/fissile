package compilator

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"
	"github.com/hpcloud/fissile/util"

	"github.com/hpcloud/termui"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	dockerImageEnvVar      = "FISSILE_TEST_DOCKER_IMAGE"
	defaultDockerTestImage = "ubuntu:14.04"
)

var dockerImageName string

var ui = termui.New(
	os.Stdin,
	ioutil.Discard,
	nil,
)

func TestMain(m *testing.M) {
	dockerImageName = os.Getenv(dockerImageEnvVar)
	if dockerImageName == "" {
		dockerImageName = defaultDockerTestImage
	}

	retCode := m.Run()

	os.Exit(retCode)
}

func TestCompilationEmpty(t *testing.T) {
	assert := assert.New(t)

	c, err := NewCompilator(nil, "", "", "", "", false, ui)
	assert.Nil(err)

	waitCh := make(chan struct{})
	go func() {
		err := c.Compile(1, genTestCase(), nil)
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

	c, err := NewCompilator(nil, "", "", "", "", false, ui)
	assert.Nil(err)

	release := genTestCase("ruby-2.5", "consul>go-1.4", "go-1.4")

	waitCh := make(chan struct{})
	go func() {
		c.Compile(1, release, nil)
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

	c, err := NewCompilator(nil, "", "", "", "", false, ui)
	assert.Nil(err)

	release := genTestCase("ruby-2.5", "consul>go-1.4", "go-1.4")

	waitCh := make(chan struct{})
	go func() {
		c.Compile(1, release, nil)
		close(waitCh)
	}()

	assert.Equal(<-compileChan, "go-1.4")
	assert.Equal(<-compileChan, "consul")
	<-waitCh
}

func TestCompilationRoleManifest(t *testing.T) {
	saveCompilePackage := compilePackageHarness
	defer func() {
		compilePackageHarness = saveCompilePackage
	}()

	compileChan := make(chan string, 2)
	compilePackageHarness = func(c *Compilator, pkg *model.Package) error {
		compileChan <- pkg.Name
		return nil
	}

	assert := assert.New(t)

	c, err := NewCompilator(nil, "", "", "", "", false, ui)
	assert.NoError(err)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)
	// This release has 3 packages:
	// `tor` is in the role manifest, and will be included
	// `libevent` is a dependency of `tor`, and will be included even though it's not in the role manifest
	// `boguspackage` is neither, and will not be included

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	roleManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)
	assert.NotNil(roleManifest)

	waitCh := make(chan struct{})
	errCh := make(chan error)
	go func() {
		errCh <- c.Compile(1, release, roleManifest)
	}()
	go func() {
		// `libevent` is a dependency of `tor` and will be compiled first
		assert.NoError(<-errCh)
		assert.Equal(<-compileChan, "libevent")
		assert.Equal(<-compileChan, "tor")
		close(waitCh)
	}()

	select {
	case <-waitCh:
		return
	case <-time.After(5 * time.Second):
		assert.Fail("Test timeout")
	}
}

func getContainerIDs(testRepository string) (string, error) {
	val, err := exec.Command("docker", "ps", "-a", "--format={{.ID}}/{{.Image}}").CombinedOutput()
	if err != nil {
		return "", err
	}
	idsOfInterest := []string{}
	slashIdx := 12 // length of an ID
	postSlashIdx := slashIdx + 1
	strVal := string(val)
	for len(strVal) > 0 {
		eolIdx := strings.Index(strVal, "\n")
		if strings.Index(strVal[postSlashIdx:eolIdx], testRepository) == 0 {
			idsOfInterest = append(idsOfInterest, strVal[0:slashIdx])
		}
		strVal = strVal[eolIdx+1:]
	}
	return strings.Join(idsOfInterest, "\n"), nil
}

func TestContainerKeptAfterCompilationWithErrors(t *testing.T) {
	doTestContainerKeptAfterCompilationWithErrors(t, true)
	doTestContainerKeptAfterCompilationWithErrors(t, false)
}

func doTestContainerKeptAfterCompilationWithErrors(t *testing.T, keepInContainer bool) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()

	releasePath := filepath.Join(workDir, "../test-assets/corrupt-releases/corrupt-package")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.Nil(err)

	testRepository := fmt.Sprintf("fissile-test-compilator-%s", uuid.New())

	comp, err := NewCompilator(dockerManager, compilationWorkDir, testRepository, compilation.FakeBase, "3.14.15", keepInContainer, ui)
	assert.Nil(err)

	imageName := comp.BaseImageName()

	_, err = comp.CreateCompilationBase(dockerImageName)
	defer func() {
		err = dockerManager.RemoveImage(imageName)
		assert.Nil(err)
	}()
	assert.Nil(err)
	beforeCompileContainers, err := getContainerIDs("fissile-test-compilator")
	assert.Nil(err)

	comp.BaseType = compilation.FailBase
	err = comp.compilePackage(release.Packages[0])
	// We expect the package to fail this time.
	assert.NotNil(err)
	afterCompileContainers, err := getContainerIDs("fissile-test-compilator")
	assert.Nil(err)

	// If keepInContainer is on,
	// We expect one more container, so we'll need to explicitly
	// remove it so the deferred func can call dockerManager.RemoveImage

	// Because all container IDs are the same length and separated
	// by newlines in the string, we can use a simple strings.Contains
	// do find instances of one container in the other.
	beforeIDs := strings.Split(beforeCompileContainers, "\n")
	droppedIDs := []string{}
	for _, id := range beforeIDs {
		if !strings.Contains(afterCompileContainers, id) {
			droppedIDs = append(droppedIDs, id)
		}
	}
	assert.Equal(len(droppedIDs), 0, fmt.Sprintf("%d IDs were dropped during the failed compile", len(droppedIDs)))

	afterIDs := strings.Split(afterCompileContainers, "\n")
	addedIDs := []string{}
	for _, id := range afterIDs {
		if !strings.Contains(beforeCompileContainers, id) {
			addedIDs = append(addedIDs, id)
		}
	}
	if keepInContainer {
		assert.Equal(1, len(addedIDs))
	} else {
		assert.Equal(0, len(addedIDs))
	}
	if keepInContainer && len(addedIDs) == 1 {
		// Do this so the deferred function that will remove
		// the container's image will succeed.
		// Sometimes the sleep command needs an explicit stop before
		// removing the container.
		err = exec.Command("docker", "stop", addedIDs[0]).Run()
		assert.Nil(err)
		err = dockerManager.RemoveContainer(addedIDs[0])
		assert.Nil(err)
	}
}

// TestCompilationMultipleErrors checks that we correctly deal with multiple compilations failing
func TestCompilationMultipleErrors(t *testing.T) {
	saveCompilePackage := compilePackageHarness
	saveIsPackageCompiled := isPackageCompiledHarness
	defer func() {
		compilePackageHarness = saveCompilePackage
		isPackageCompiledHarness = saveIsPackageCompiled
	}()

	compilePackageHarness = func(c *Compilator, pkg *model.Package) error {
		return fmt.Errorf("Intentional error compiling %s", pkg.Name)
	}

	isPackageCompiledHarness = func(c *Compilator, pkg *model.Package) (bool, error) {
		return false, nil
	}

	assert := assert.New(t)

	c, err := NewCompilator(nil, "", "", "", "", false, ui)
	assert.Nil(err)

	release := genTestCase("ruby-2.5", "consul>go-1.4", "go-1.4")

	err = c.Compile(1, release, nil)
	assert.NotNil(err)
}

func TestGetPackageStatusCompiled(t *testing.T) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := model.NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test-compilator", compilation.FakeBase, "3.14.15", false, ui)
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
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := model.NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test-compilator", compilation.FakeBase, "3.14.15", false, ui)
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
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := model.NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test-compilator", compilation.FakeBase, "3.14.15", false, ui)
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
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := model.NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.Nil(err)

	compilator, err := NewCompilator(dockerManager, compilationWorkDir, "fissile-test-compilator", compilation.FakeBase, "3.14.15", false, ui)
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
	doTestCompilePackage(t, true)
	doTestCompilePackage(t, false)
}

func doTestCompilePackage(t *testing.T, keepInContainer bool) {
	assert := assert.New(t)

	compilationWorkDir, err := util.TempDir("", "fissile-tests")
	assert.Nil(err)
	defer os.RemoveAll(compilationWorkDir)

	dockerManager, err := docker.NewImageManager()
	assert.Nil(err)

	workDir, err := os.Getwd()
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := model.NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	testRepository := fmt.Sprintf("fissile-test-compilator-%s", uuid.New())

	comp, err := NewCompilator(dockerManager, compilationWorkDir, testRepository, compilation.FakeBase, "3.14.15", keepInContainer, ui)
	assert.Nil(err)

	imageName := comp.BaseImageName()

	_, err = comp.CreateCompilationBase(dockerImageName)
	defer func() {
		err = dockerManager.RemoveImage(imageName)
		assert.Nil(err)
	}()
	assert.Nil(err)
	beforeCompileContainers, err := getContainerIDs("fissile-test-compilator")
	assert.Nil(err)

	err = comp.compilePackage(release.Packages[0])
	assert.Nil(err)
	afterCompileContainers, err := getContainerIDs("fissile-test-compilator")
	assert.Nil(err)
	assert.Equal(string(beforeCompileContainers), string(afterCompileContainers))
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
	assert.Equal(t, len(buckets), 4)
	assert.Equal(t, buckets[0].Name, "ruby-2.5") // Ruby should be first
	assert.Equal(t, buckets[1].Name, "go-1.4")
	assert.Equal(t, buckets[2].Name, "consul")
	assert.Equal(t, buckets[3].Name, "cloud_controller_go")
}

func TestCreateDepBucketsOnChain(t *testing.T) {
	t.Parallel()

	packages := []*model.Package{
		{Name: "A", Dependencies: nil},
		{Name: "B", Dependencies: []*model.Package{{Name: "C"}}},
		{Name: "C", Dependencies: []*model.Package{{Name: "A"}}},
	}

	buckets := createDepBuckets(packages)
	assert.Equal(t, len(buckets), 3)
	assert.Equal(t, buckets[0].Name, "A")
	assert.Equal(t, buckets[1].Name, "C")
	assert.Equal(t, buckets[2].Name, "B")
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

	c, err := NewCompilator(nil, "", "", "", "", false, ui)
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
