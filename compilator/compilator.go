package compilator

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"
	"github.com/hpcloud/fissile/util"

	"github.com/fatih/color"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/go-multierror"
	"github.com/termie/go-shutil"
)

const (
	// ContainerPackagesDir represents the default location of installed BOSH packages
	ContainerPackagesDir = "/var/vcap/packages"

	sleepTimeWhileCantCompileSec = 5
)

const (
	packageError = iota
	packageNone
	packageCompiling
	packageCompiled
)

// Compilator represents the BOSH compiler
type Compilator struct {
	DockerManager    *docker.ImageManager
	HostWorkDir      string
	RepositoryPrefix string
	BaseType         string
	FissileVersion   string

	packageLock      map[*model.Package]*sync.Mutex
	packageCompiling map[*model.Package]bool
}

// NewCompilator will create an instance of the Compilator
func NewCompilator(
	dockerManager *docker.ImageManager,
	hostWorkDir string,
	repositoryPrefix string,
	baseType string,
	fissileVersion string,
) (*Compilator, error) {

	compilator := &Compilator{
		DockerManager:    dockerManager,
		HostWorkDir:      hostWorkDir,
		RepositoryPrefix: repositoryPrefix,
		BaseType:         baseType,
		FissileVersion:   fissileVersion,

		packageLock:      map[*model.Package]*sync.Mutex{},
		packageCompiling: map[*model.Package]bool{},
	}

	return compilator, nil
}

// Compile will perform the compile of a BOSH release
func (c *Compilator) Compile(workerCount int, release *model.Release) error {
	var result error

	c.initPackageMaps(release)

	// TODO Check for cycles

	// Iterate until all packages are compiled
	// Not the most efficient implementation,
	// but it's easy to parallelize and reason about
	var workersGroup sync.WaitGroup

	for idx := 0; idx < workerCount; idx++ {
		workersGroup.Add(1)

		go func(workerIdx int) {
			defer workersGroup.Done()

			hasWork := false
			done := false

			for done == false {
				log.Printf("worker-%s > Compilation work started.\n", color.YellowString("%d", workerIdx))

				hasWork = false

				for _, pkg := range release.Packages {
					pkgState, workerErr := c.getPackageStatus(pkg)
					if workerErr != nil {
						result = multierror.Append(result, workerErr)
						return
					}

					if pkgState == packageNone {
						func() {
							// Set package state to "compiling"
							func() {
								defer func() {
									c.packageLock[pkg].Unlock()
								}()
								c.packageLock[pkg].Lock()
								c.packageCompiling[pkg] = true
							}()
							// Set package state to "not compiling" when done
							defer func() {
								defer func() {
									c.packageLock[pkg].Unlock()
								}()
								c.packageLock[pkg].Lock()
								c.packageCompiling[pkg] = false
							}()
							if pkgState == packageNone {
								compiled, workerErr := c.compilePackage(pkg)
								hasWork = compiled
								if workerErr != nil {
									log.Println(color.RedString(
										"worker-%s > Compiling package %s failed: %s.\n",
										color.YellowString("%d", workerIdx),
										color.GreenString(pkg.Name),
										color.RedString(workerErr.Error()),
									))
									result = multierror.Append(result, workerErr)
								}
							}
						}()

						if result != nil {
							break
						}
					}
				}

				if result != nil {
					break
				}

				done = true

				for _, pkg := range release.Packages {
					pkgState, workerErr := c.getPackageStatus(pkg)
					if workerErr != nil {
						result = multierror.Append(result, workerErr)
						return
					}

					if pkgState != packageCompiled {
						done = false
						break
					}
				}

				// Wait a bit if there's nothing to work on
				if !done && !hasWork {
					log.Printf("worker-%s > Didn't find any work, sleeping ...\n", color.YellowString("%d", workerIdx))
					time.Sleep(sleepTimeWhileCantCompileSec * time.Second)
				}
			}
		}(idx)
	}

	// Wait until all workers finish
	workersGroup.Wait()

	return result
}

// CreateCompilationBase will create the compiler container
func (c *Compilator) CreateCompilationBase(baseImageName string) (image *dockerClient.Image, err error) {
	imageTag := c.BaseCompilationImageTag()
	imageName := c.BaseImageName()
	log.Println(color.GreenString("Using %s as a compilation image name", color.YellowString(imageName)))

	containerName := c.baseCompilationContainerName()
	log.Println(color.GreenString("Using %s as a compilation container name", color.YellowString(containerName)))

	image, err = c.DockerManager.FindImage(imageName)
	if err != nil {
		log.Println("Image doesn't exist, it will be created ...")
	} else {
		log.Println(color.GreenString(
			"Compilation image %s with ID %s already exists. Doing nothing.",
			color.YellowString(imageName),
			color.YellowString(image.ID),
		))
		return image, nil
	}

	tempScriptDir, err := util.TempDir("", "fissile-compilation")
	if err != nil {
		return nil, fmt.Errorf("Could not create temp dir %s: %s", tempScriptDir, err.Error())
	}

	targetScriptName := "compilation-prerequisites.sh"
	containerScriptPath := filepath.Join(docker.ContainerInPath, targetScriptName)
	hostScriptPath := filepath.Join(tempScriptDir, targetScriptName)
	if err = compilation.SaveScript(c.BaseType, compilation.PrerequisitesScript, hostScriptPath); err != nil {
		return nil, fmt.Errorf("Error saving script asset: %s", err.Error())
	}

	exitCode, container, err := c.DockerManager.RunInContainer(
		containerName,
		baseImageName,
		[]string{"bash", "-c", containerScriptPath},
		tempScriptDir,
		"",
		func(stdout io.Reader) {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				log.Println(color.GreenString("compilation-container > %s", color.WhiteString(scanner.Text())))
			}
		},
		func(stderr io.Reader) {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Println(color.GreenString("compilation-container > %s", color.RedString(scanner.Text())))
			}
		},
	)
	defer func() {
		if container != nil {
			removeErr := c.DockerManager.RemoveContainer(container.ID)
			if removeErr != nil {
				if err == nil {
					err = removeErr
				} else {
					err = fmt.Errorf(
						"Image creation error: %s. Image removal error: %s.",
						err,
						removeErr,
					)
				}
			}
		}
	}()

	if err != nil {
		return nil, fmt.Errorf("Error running script: %s", err.Error())
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("Error - script script exited with code %d", exitCode)
	}

	image, err = c.DockerManager.CreateImage(
		container.ID,
		c.baseCompilationImageRepository(),
		imageTag,
		"",
		[]string{},
	)

	if err != nil {
		return nil, fmt.Errorf("Error creating image %s", err.Error())
	}

	log.Println(color.GreenString(
		"Image %s with ID %s created successfully.",
		color.YellowString(c.BaseImageName()),
		color.YellowString(container.ID)))

	return image, nil
}

func (c *Compilator) compilePackage(pkg *model.Package) (compiled bool, err error) {

	// Do nothing if any dependency has not been compiled
	for _, dep := range pkg.Dependencies {

		packageStatus, err := c.getPackageStatus(dep)
		if err != nil {
			return false, err
		}

		if packageStatus != packageCompiled {
			return false, nil
		}
	}
	log.Println(color.GreenString("compilation-%s > %s", color.MagentaString(pkg.Name), color.WhiteString("Starting compilation ...")))

	// Prepare input dir (package plus deps)
	if err := c.createCompilationDirStructure(pkg); err != nil {
		return false, err
	}

	if err := c.copyDependencies(pkg); err != nil {
		return false, err
	}

	// Generate a compilation script
	targetScriptName := "compile.sh"
	hostScriptPath := filepath.Join(c.getTargetPackageSourcesDir(pkg), targetScriptName)
	containerScriptPath := filepath.Join(docker.ContainerInPath, targetScriptName)
	if err := compilation.SaveScript(c.BaseType, compilation.CompilationScript, hostScriptPath); err != nil {
		return false, err
	}

	// Extract package
	extractDir := c.getSourcePackageDir(pkg)
	if _, err := pkg.Extract(extractDir); err != nil {
		return false, err
	}

	// Run compilation in container
	containerName := c.getPackageContainerName(pkg)
	exitCode, container, err := c.DockerManager.RunInContainer(
		containerName,
		c.BaseImageName(),
		[]string{"bash", containerScriptPath, pkg.Name, pkg.Version},
		c.getTargetPackageSourcesDir(pkg),
		c.getPackageCompiledTempDir(pkg),
		func(stdout io.Reader) {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				log.Println(color.GreenString("compilation-%s > %s", color.MagentaString(pkg.Name), color.WhiteString(scanner.Text())))
			}
		},
		func(stderr io.Reader) {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Println(color.GreenString("compilation-%s > %s", color.MagentaString(pkg.Name), color.RedString(scanner.Text())))
			}
		},
	)
	defer func() {
		// Remove container
		if container != nil {
			if removeErr := c.DockerManager.RemoveContainer(container.ID); removeErr != nil {
				if err == nil {
					err = removeErr
				} else {
					err = fmt.Errorf("Error compiling package: %s. Error removing package: %s", err.Error(), removeErr.Error())
				}
			}
		}
	}()

	if err != nil {
		return false, err
	}

	if exitCode != 0 {
		return false, fmt.Errorf("Error - compilation for package %s exited with code %d", pkg.Name, exitCode)
	}

	// It all went ok - we can move the compiled-temp bits to the compiled dir
	os.Rename(c.getPackageCompiledTempDir(pkg), c.getPackageCompiledDir(pkg))

	return true, nil
}

// We want to create a package structure like this:
// .
// └── <pkg-name>
//     ├── compiled
//     ├── compiled-temp
//     └── sources
//         └── var
//             └── vcap
//                 ├── packages
//                 │   └── <dependency-package>
//                 └── source
func (c *Compilator) createCompilationDirStructure(pkg *model.Package) error {
	dependenciesPackageDir := c.getDependenciesPackageDir(pkg)
	sourcePackageDir := c.getSourcePackageDir(pkg)

	if err := os.MkdirAll(dependenciesPackageDir, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(sourcePackageDir, 0755); err != nil {
		return err
	}

	return nil
}

func (c *Compilator) getTargetPackageSourcesDir(pkg *model.Package) string {
	return filepath.Join(c.HostWorkDir, pkg.Name, "sources")
}

func (c *Compilator) getPackageCompiledTempDir(pkg *model.Package) string {
	return filepath.Join(c.HostWorkDir, pkg.Name, "compiled-temp")
}

func (c *Compilator) getPackageCompiledDir(pkg *model.Package) string {
	return filepath.Join(c.HostWorkDir, pkg.Name, "compiled")
}

func (c *Compilator) getDependenciesPackageDir(pkg *model.Package) string {
	return filepath.Join(c.getTargetPackageSourcesDir(pkg), "var", "vcap", "packages")
}

func (c *Compilator) getSourcePackageDir(pkg *model.Package) string {
	return filepath.Join(c.getTargetPackageSourcesDir(pkg), "var", "vcap", "source")
}

func (c *Compilator) copyDependencies(pkg *model.Package) error {
	for _, dep := range pkg.Dependencies {
		depCompiledPath := c.getPackageCompiledDir(dep)
		depDestinationPath := filepath.Join(c.getDependenciesPackageDir(pkg), dep.Name)
		if err := os.RemoveAll(depDestinationPath); err != nil {
			return err
		}

		if err := shutil.CopyTree(
			depCompiledPath,
			depDestinationPath,
			&shutil.CopyTreeOptions{
				Symlinks:               true,
				Ignore:                 nil,
				CopyFunction:           shutil.Copy,
				IgnoreDanglingSymlinks: false},
		); err != nil {
			return err
		}
	}

	return nil
}

func (c *Compilator) getPackageStatus(pkg *model.Package) (int, error) {
	// Acquire mutex before checking status
	c.packageLock[pkg].Lock()
	defer func() {
		c.packageLock[pkg].Unlock()
	}()

	// If package is in packageCompiling hash
	if c.packageCompiling[pkg] {
		return packageCompiling, nil
	}

	// If compiled package exists on hard
	compiledPackagePath := filepath.Join(c.HostWorkDir, pkg.Name, "compiled")
	compiledPackagePathExists, err := validatePath(compiledPackagePath, true, "package path")
	if err != nil {
		return packageError, err
	}

	if compiledPackagePathExists {
		compiledDirEmpty, err := isDirEmpty(compiledPackagePath)
		if err != nil {
			return packageError, err
		}

		if !compiledDirEmpty {
			return packageCompiled, nil
		}
	}

	// Package is in no state otherwise
	return packageNone, nil
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return true, err
	}

	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}

func validatePath(path string, shouldBeDir bool, pathDescription string) (bool, error) {
	pathInfo, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	if pathInfo.IsDir() && !shouldBeDir {
		return false, fmt.Errorf("Path %s (%s) points to a directory. It should be a a file.", path, pathDescription)
	} else if !pathInfo.IsDir() && shouldBeDir {
		return false, fmt.Errorf("Path %s (%s) points to a file. It should be a directory.", path, pathDescription)
	}

	return true, nil
}

// baseCompilationContainerName will return the compilation container's name
func (c *Compilator) baseCompilationContainerName() string {
	return fmt.Sprintf("%s-%s", c.baseCompilationImageRepository(), c.FissileVersion)
}

func (c *Compilator) getPackageContainerName(pkg *model.Package) string {
	return fmt.Sprintf("%s-%s-%s-pkg-%s", c.baseCompilationContainerName(), pkg.Release.Name, pkg.Release.Version, pkg.Name)
}

// BaseCompilationImageTag will return the compilation image tag
func (c *Compilator) BaseCompilationImageTag() string {
	return fmt.Sprintf("%s", c.FissileVersion)
}

// baseCompilationImageRepository will return the compilation image repository
func (c *Compilator) baseCompilationImageRepository() string {
	return fmt.Sprintf("%s-cbase", c.RepositoryPrefix)
}

// BaseImageName returns the name of the compilation base image
func (c *Compilator) BaseImageName() string {
	return fmt.Sprintf("%s:%s", c.baseCompilationImageRepository(), c.BaseCompilationImageTag())
}

func (c *Compilator) initPackageMaps(release *model.Release) {
	for _, pkg := range release.Packages {
		c.packageLock[pkg] = &sync.Mutex{}
		c.packageCompiling[pkg] = false
	}
}
