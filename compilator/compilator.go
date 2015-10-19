package compilator

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"
	"github.com/hpcloud/fissile/util"

	"github.com/fatih/color"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/termie/go-shutil"
)

const (
	// ContainerPackagesDir represents the default location of installed BOSH packages
	ContainerPackagesDir = "/var/vcap/packages"
)

// Compilator represents the BOSH compiler
type Compilator struct {
	DockerManager    *docker.ImageManager
	HostWorkDir      string
	RepositoryPrefix string
	BaseType         string
	FissileVersion   string

	packageDone map[string]chan struct{}
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

		packageDone: make(map[string]chan struct{}),
	}

	return compilator, nil
}

var errWorkerAbort = errors.New("worker aborted")

type compileResult struct {
	Pkg *model.Package
	Err error
}

// Compile concurrency works like this:
// 1 routine producing (todoCh<-)
// n workers consuming (<-todoCh)
// 1 synchronizer consuming EXACTLY 1 <-doneCh for every <-todoCh
//
// Dependencies:
// Packages with the least dependencies are queued first
// workers wait for their dependencies by waiting on a map of broadcasting
// channels that are closed by the synchronizer when something is done
// compiling successfully
//
// In the event of an error:
// - workers will try to bail out of waiting on <-todo or <-c.packageDone[name]
//   early if it finds the killCh has been activated. There is a "race" here
//   to see if the synchronizer will drain <-todoCh or if they will select on
//   <-killCh before <-todoCh. In the worst case, an extra package will be
//   compiled by each active worker.
// - synchronizer will greedily drain the <-todoCh to starve the workers out
//   and won't wait for the <-doneCh for the N packages it drained.
func (c *Compilator) Compile(workerCount int, release *model.Release) error {
	c.initPackageMaps(release)
	var workersGroup sync.WaitGroup

	todoCh := make(chan *model.Package)
	doneCh := make(chan compileResult)
	killCh := make(chan struct{})

	for i := 0; i < workerCount; i++ {
		workersGroup.Add(1)
		go c.compileWorker(i, todoCh, doneCh, killCh, &workersGroup)
	}

	go func() {
		workersGroup.Add(1)
		buckets := createDepBuckets(release.Packages)
		for _, bucketPackages := range buckets {
			for _, pkg := range bucketPackages {
				todoCh <- pkg
			}
		}
		close(todoCh)
		workersGroup.Done()
	}()

	nPackages := len(release.Packages)
	killed := false
	for nPackages > 0 {
		result := <-doneCh
		nPackages--

		if result.Err == nil {
			close(c.packageDone[result.Pkg.Name])
			log.Printf("%s   > success: %s\n",
				color.YellowString("result"), color.GreenString(result.Pkg.Name))
			continue
		}

		log.Printf(
			"%s   > failure: %s - %s\n",
			color.YellowString("result"),
			color.RedString(result.Pkg.Name),
			color.RedString(result.Err.Error()),
		)

		if !killed {
			killed = true
			close(killCh)
			// Drain the todo channel.
			nTodo := 0
			for range todoCh {
				nTodo++
			}
			nPackages -= nTodo
		}
	}

	workersGroup.Wait()

	return nil
}

func (c *Compilator) compileWorker(
	id int,
	todoCh chan *model.Package,
	doneCh chan compileResult,
	killCh chan struct{},
	wg *sync.WaitGroup,
) {

	log.Printf("worker-%s > start\n", color.YellowString("%d", id))

MainLoop:
	for {
		var pkg *model.Package
		var ok bool

		select {
		case <-killCh:
			log.Printf("worker-%s > killed", color.YellowString("%d", id))
			break MainLoop
		case pkg, ok = <-todoCh:
			if !ok {
				break MainLoop
			}
		}

		// Wait for our deps
		for _, dep := range pkg.Dependencies {
			done := false
			for !done {
				select {
				case <-killCh:
					log.Printf("worker-%s > killed:  %s",
						color.YellowString("%d", id), color.MagentaString(pkg.Name))
					doneCh <- compileResult{Pkg: pkg, Err: errWorkerAbort}
					break MainLoop
				case <-time.After(5 * time.Second):
					log.Printf("worker-%s > waiting: %s - %s",
						color.YellowString("%d", id), color.MagentaString(pkg.Name), color.MagentaString(dep.Name))
				case <-c.packageDone[dep.Name]:
					log.Printf("worker-%s > depdone: %s - %s",
						color.YellowString("%d", id), color.MagentaString(pkg.Name), color.MagentaString(dep.Name))
					done = true
				}
			}
		}

		log.Printf("worker-%s > compile: %s\n", color.YellowString("%d", id), color.MagentaString(pkg.Name))

		workerErr := c.compilePackage(pkg)
		log.Printf("worker-%s > done:    %s\n", color.YellowString("%d", id), color.MagentaString(pkg.Name))

		doneCh <- compileResult{Pkg: pkg, Err: workerErr}
	}

	log.Printf("worker-%s > exit\n", color.YellowString("%d", id))
	wg.Done()
}

func createDepBuckets(packages []*model.Package) [][]*model.Package {
	var buckets [][]*model.Package

	// ruby takes forever and has no deps,
	// so optimize by moving ruby packages to the front
	var rubies []*model.Package

	for _, pkg := range packages {
		depCount := len(pkg.Dependencies)
		for depCount >= len(buckets) {
			buckets = append(buckets, nil)
		}

		if strings.HasPrefix(pkg.Name, "ruby-2.") {
			rubies = append(rubies, pkg)
			continue
		}

		buckets[depCount] = append(buckets[depCount], pkg)
	}

	// prepend rubies to get them out of the way first
	bucket0 := buckets[0]
	bucket0 = append(bucket0, rubies...)
	for i := range rubies {
		bucket0[i], bucket0[len(bucket0)-i-1] =
			bucket0[len(bucket0)-i-1], bucket0[i]
	}
	buckets[0] = bucket0

	return buckets
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

func (c *Compilator) compilePackage(pkg *model.Package) (err error) {
	//log.Println(color.GreenString("compilation-%s > %s", color.MagentaString(pkg.Name), color.WhiteString("Starting compilation ...")))

	// Prepare input dir (package plus deps)
	if err := c.createCompilationDirStructure(pkg); err != nil {
		return err
	}

	if err := c.copyDependencies(pkg); err != nil {
		return err
	}

	// Generate a compilation script
	targetScriptName := "compile.sh"
	hostScriptPath := filepath.Join(c.getTargetPackageSourcesDir(pkg), targetScriptName)
	containerScriptPath := filepath.Join(docker.ContainerInPath, targetScriptName)
	if err := compilation.SaveScript(c.BaseType, compilation.CompilationScript, hostScriptPath); err != nil {
		return err
	}

	// Extract package
	extractDir := c.getSourcePackageDir(pkg)
	if _, err := pkg.Extract(extractDir); err != nil {
		return err
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
				//log.Println(color.GreenString("compilation-%s > %s", color.MagentaString(pkg.Name), color.WhiteString(scanner.Text())))
			}
		},
		func(stderr io.Reader) {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				//log.Println(color.GreenString("compilation-%s > %s", color.MagentaString(pkg.Name), color.RedString(scanner.Text())))
			}
		},
	)

	defer func() {
		// Remove container
		if container == nil {
			return
		}

		if removeErr := c.DockerManager.RemoveContainer(container.ID); removeErr != nil {
			if err == nil {
				err = removeErr
			} else {
				err = fmt.Errorf("Error compiling package: %s. Error removing package: %s", err.Error(), removeErr.Error())
			}
		}
	}()

	if err != nil {
		return err
	}

	if exitCode != 0 {
		return fmt.Errorf("Error - compilation for package %s exited with code %d", pkg.Name, exitCode)
	}

	return os.Rename(c.getPackageCompiledTempDir(pkg), c.getPackageCompiledDir(pkg))
}

// createComplilationDirStructure creates a package structure like this:
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
		c.packageDone[pkg.Name] = make(chan struct{})
	}
}
