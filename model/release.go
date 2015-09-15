package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Release struct {
	Jobs               []*Job
	Packages           []*Package
	License            ReleaseLicense
	Name               string
	UncommittedChanges bool
	CommitHash         string
	Version            string
	Path               string

	manifest map[interface{}]interface{}
}

const (
	jobsDir        = "jobs"
	packagesDir    = "packages"
	licenseArchive = "license.tgz"
	manifestFile   = "release.MF"
)

func NewRelease(path string) (*Release, error) {

	release := &Release{
		Path:     path,
		Packages: []*Package{},
		Jobs:     []*Job{},
		License:  ReleaseLicense{},
	}

	if err := release.validatePathStructure(); err != nil {
		return nil, err
	}

	if err := release.loadMetadata(); err != nil {
		return nil, err
	}

	if err := release.loadPackages(); err != nil {
		return nil, err
	}

	if err := release.loadJobs(); err != nil {
		return nil, err
	}

	return release, nil
}

func (r *Release) loadMetadata() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error trying to load release metadata from YAML manifest: %s", r)
		}
	}()

	manifestContents, err := ioutil.ReadFile(r.manifestFilePath())
	if err != nil {
		return err
	}

	err = yaml.Unmarshal([]byte(manifestContents), &r.manifest)

	r.CommitHash = r.manifest["commit_hash"].(string)
	r.UncommittedChanges = r.manifest["uncommitted_changes"].(bool)
	r.Name = r.manifest["name"].(string)
	r.Version = r.manifest["version"].(string)

	return nil
}

func (r *Release) lookupPackage(packageName string) (*Package, error) {
	for _, pkg := range r.Packages {
		if pkg.Name == packageName {
			return pkg, nil
		}
	}

	return nil, fmt.Errorf("Cannot find package %s in release", packageName)
}

func (r *Release) loadJobs() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error trying to load release jobs from YAML manifest: %s", r)
		}
	}()

	jobs := r.manifest["jobs"].([]interface{})
	for _, job := range jobs {
		j, err := newJob(r, job.(map[interface{}]interface{}))
		if err != nil {
			return err
		}

		r.Jobs = append(r.Jobs, j)
	}

	return nil
}

func (r *Release) loadPackages() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error trying to load release packages from YAML manifest: %s", r)
		}
	}()

	packages := r.manifest["packages"].([]interface{})
	for _, pkg := range packages {
		p, err := newPackage(r, pkg.(map[interface{}]interface{}))
		if err != nil {
			return err
		}

		r.Packages = append(r.Packages, p)
	}

	return nil
}

func (r *Release) loadLicense() error {
	return nil
}

func (r *Release) validatePathStructure() error {
	if err := validatePath(r.Path, true, "release directory"); err != nil {
		return err
	}

	if err := validatePath(r.manifestFilePath(), false, "release manifest file"); err != nil {
		return err
	}

	if err := validatePath(r.packagesDirPath(), true, "packages directory"); err != nil {
		return err
	}

	if err := validatePath(r.jobsDirPath(), true, "jobs directory"); err != nil {
		return err
	}

	if err := validatePath(r.licenseArchivePath(), false, "license archive file"); err != nil {
		return err
	}

	return nil
}

func validatePath(path string, shouldBeDir bool, pathDescription string) error {
	pathInfo, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Path %s (%s) does not exist.", path, pathDescription)
		} else {
			return err
		}
	}

	if pathInfo.IsDir() && !shouldBeDir {
		return fmt.Errorf("Path %s (%s) points to a directory. It should be a a file. %s", path, pathDescription)
	} else if !pathInfo.IsDir() && shouldBeDir {
		return fmt.Errorf("Path %s (%s) points to a file. It should be a directory.", path, pathDescription)
	}

	return nil
}

func (r *Release) licenseArchivePath() string {
	return filepath.Join(r.Path, licenseArchive)
}

func (r *Release) packagesDirPath() string {
	return filepath.Join(r.Path, packagesDir)
}

func (r *Release) jobsDirPath() string {
	return filepath.Join(r.Path, jobsDir)
}

func (r *Release) manifestFilePath() string {
	return filepath.Join(r.Path, manifestFile)
}
