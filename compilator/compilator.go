package compilator

import (
	)
	
const (
	ContainerPackagesDir = "/var/vcap/packages"
	)
	
const (
	packageNone = iota
	packageCompiling
	packageCompiled
)
	
type Compilator struct {
	DockerManager *docker.DockerImageManager
	Release *model.Release
	HostWorkDir string
	
	packageLock []sync.Mutex
	packageCompiling hash[string]false
}


func NewCompilator(dockerManager *docker.DockerImageManager, release *model.Release, hostWorkDir string) (*Compilator, error) {
	compilator := &Compilator {
		DockerManager: dockerManager,
		Release: release,
		HostWorkDir: hostWorkDir,
	}
	
	return compilator, nil
}

func (c *Compilator) Compile(workerCount int) error {
	// Check for cycles
	
	// Iterate until all packages are compiled
	// Not the most efficient implementation, 
	// but it's easy to parallelize and reason about
	for workerIdx := 0; workerIdx < workerCount; workerIdx++ {
		go func (idx int) {
			// Wait a bit if there's nothing to work on
		}(workerIdx)
	}
	
	// Wait until all workers finish
}

func (c *Compilator) compilePackage(model.Package pkg) error {
	// Do nothing if any dependency has not been compiled
	for _, dep := range pkg.Dependencies {
		if getPackageStatus(dep) != packageCompiled {
			return nil
		}
	}
	
	// Compile
	// Prepare input dir (package plus deps), generate a compilation script, run compilation in container,
	// remove container, copy compiled package to its destination in the work dir
}

func (c *Compilator) getPackageStatus(mode.Package pkg) (int, error) {
	// Acquire mutex before checking status
	
	// If package is in packageCompiling hash
	return packageCompiling, nil
	
	// If compiled package exists on hard
	return packageCompiled, nil
	
	// Package is in no state otherwise
	return packageNone, nil
}