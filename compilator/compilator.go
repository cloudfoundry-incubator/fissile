package compilator

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"

	"github.com/crufter/copyrecur"
)

const (
	ContainerPackagesDir = "/var/vcap/packages"
)

const (
	packageError = iota
	packageNone
	packageCompiling
	packageCompiled
)

type Compilator struct {
	DockerManager *docker.DockerImageManager
	Release       *model.Release
	HostWorkDir   string

	packageLock      map[*model.Package]*sync.Mutex
	packageCompiling map[*model.Package]bool
}

func NewCompilator(dockerManager *docker.DockerImageManager, release *model.Release, hostWorkDir string) (*Compilator, error) {
	compilator := &Compilator{
		DockerManager: dockerManager,
		Release:       release,
		HostWorkDir:   hostWorkDir,

		packageLock:      map[*model.Package]*sync.Mutex{},
		packageCompiling: map[*model.Package]bool{},
	}

	for _, pkg := range release.Packages {
		compilator.packageLock[pkg] = &sync.Mutex{}
		compilator.packageCompiling[pkg] = false
	}

	return compilator, nil
}

func (c *Compilator) Compile(workerCount int) error {
	// Check for cycles

	// Iterate until all packages are compiled
	// Not the most efficient implementation,
	// but it's easy to parallelize and reason about
	for workerIdx := 0; workerIdx < workerCount; workerIdx++ {
		go func(idx int) {
			// Wait a bit if there's nothing to work on
		}(workerIdx)
	}

	// Wait until all workers finish

	return nil
}

func (c *Compilator) compilePackage(pkg *model.Package) error {
	// Do nothing if any dependency has not been compiled
	for _, dep := range pkg.Dependencies {

		packageStatus, err := c.getPackageStatus(dep)
		if err != nil {
			return err
		}

		if packageStatus != packageCompiled {
			return nil
		}
	}

	// Compile
	// Prepare input dir (package plus deps), generate a compilation script, run compilation in container,
	// remove container, copy compiled package to its destination in the work dir

	if err := c.createCompilationDirStructure(pkg); err != nil {
		return err
	}

	if err := c.copyDependencies(pkg); err != nil {
		return err
	}

	return nil
}

// We want to create a package structure like this:
// .
// └── <pkg-name>
//     ├── compiled
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
		if err := copyrecur.CopyDir(depCompiledPath, depDestinationPath); err != nil {
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
	compiledPackageExists, err := validatePath(compiledPackagePath, true, "package path")
	if err != nil {
		return packageError, err
	}

	if compiledPackageExists {
		return packageCompiled, nil
	}

	// Package is in no state otherwise
	return packageNone, nil
}

func validatePath(path string, shouldBeDir bool, pathDescription string) (bool, error) {
	pathInfo, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	if pathInfo.IsDir() && !shouldBeDir {
		return false, fmt.Errorf("Path %s (%s) points to a directory. It should be a a file. %s", path, pathDescription)
	} else if !pathInfo.IsDir() && shouldBeDir {
		return false, fmt.Errorf("Path %s (%s) points to a file. It should be a directory.", path, pathDescription)
	}

	return true, nil
}
