package model

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hpcloud/fissile/util"

	"gopkg.in/yaml.v2"
)

// Release represents a BOSH release
type Release struct {
	Jobs               []*Job
	Packages           []*Package
	License            ReleaseLicense
	Notice             ReleaseLicense
	Name               string
	UncommittedChanges bool
	CommitHash         string
	Version            string
	Path               string
	Dev                bool
	DevBOSHCacheDir    string

	manifest map[interface{}]interface{}
}

const (
	jobsDir        = "jobs"
	packagesDir    = "packages"
	licenseArchive = "license.tgz"
	manifestFile   = "release.MF"
)

// NewRelease will create an instance of a BOSH release
func NewRelease(path string) (*Release, error) {
	release := &Release{
		Path:     path,
		Packages: []*Package{},
		Jobs:     []*Job{},
		License:  ReleaseLicense{},
		Dev:      false,
	}

	if err := release.validatePathStructure(); err != nil {
		return nil, err
	}

	if err := release.loadMetadata(); err != nil {
		return nil, err
	}

	if err := release.loadLicense(); err != nil {
		return nil, err
	}

	if err := release.loadPackages(); err != nil {
		return nil, err
	}

	if err := release.loadDependenciesForPackages(); err != nil {
		return nil, err
	}

	if err := release.loadJobs(); err != nil {
		return nil, err
	}

	return release, nil
}

// GetUniqueConfigs returns all unique configs available in a release
func (r *Release) GetUniqueConfigs() map[string]*ReleaseConfig {
	result := map[string]*ReleaseConfig{}

	for _, job := range r.Jobs {
		for _, property := range job.Properties {

			if config, ok := result[property.Name]; ok {
				config.UsageCount++
				config.Jobs = append(config.Jobs, job)
			} else {
				result[property.Name] = &ReleaseConfig{
					Name:        property.Name,
					Jobs:        []*Job{job},
					UsageCount:  1,
					Description: property.Description,
				}
			}
		}
	}

	return result
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

// LookupPackage will find a package within a BOSH release
func (r *Release) LookupPackage(packageName string) (*Package, error) {
	for _, pkg := range r.Packages {
		if pkg.Name == packageName {
			return pkg, nil
		}
	}

	return nil, fmt.Errorf("Cannot find package %s in release", packageName)
}

// LookupJob will find a job within a BOSH release
func (r *Release) LookupJob(jobName string) (*Job, error) {
	for _, job := range r.Jobs {
		if job.Name == jobName {
			return job, nil
		}
	}

	return nil, fmt.Errorf("Cannot find job %s in release", jobName)
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

func (r *Release) loadDependenciesForPackages() error {
	for _, pkg := range r.Packages {
		if err := pkg.loadPackageDependencies(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Release) loadLicense() error {
	err := targzIterate(r.licenseArchivePath(),
		func(tarfile *tar.Reader, header *tar.Header) error {
			hash := sha1.New()
			licenseFile := io.TeeReader(tarfile, hash)

			licenseWriter := &bytes.Buffer{}
			if _, err := io.Copy(licenseWriter, licenseFile); err != nil {
				return fmt.Errorf("failed reading tar'd file: %s, %v", header.Name, err)
			}

			sha1hash := fmt.Sprintf("%02x", hash.Sum(nil))
			name := strings.ToLower(header.Name)

			switch {
			case strings.Contains(name, "license"):
				r.License.Contents = licenseWriter.Bytes()
				r.License.SHA1 = sha1hash
				r.License.Filename = filepath.Base(header.Name)
			case strings.Contains(name, "notice"):
				r.Notice.Contents = licenseWriter.Bytes()
				r.Notice.SHA1 = sha1hash
				r.Notice.Filename = filepath.Base(header.Name)
			}

			return nil
		})

	return err
}

func (r *Release) validatePathStructure() error {
	if err := util.ValidatePath(r.Path, true, "release directory"); err != nil {
		return err
	}

	if err := util.ValidatePath(r.manifestFilePath(), false, "release manifest file"); err != nil {
		return err
	}

	if err := util.ValidatePath(r.packagesDirPath(), true, "packages directory"); err != nil {
		return err
	}

	if err := util.ValidatePath(r.jobsDirPath(), true, "jobs directory"); err != nil {
		return err
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
	if r.Dev {
		return filepath.Join(r.getDevReleaseManifestsDir(), r.getDevReleaseManifestFilename())
	} else {
		return filepath.Join(r.Path, manifestFile)
	}
}

// targzIterate iterates over the files it finds in a tar.gz file and calls a
// callback for each file encountered.
func targzIterate(filename string, fn func(*tar.Reader, *tar.Header) error) error {
	targz, err := os.Open(filename)
	if err != nil {
		return nil // Don't care if this file doesn't exist
	}

	defer targz.Close()

	gzipReader, err := gzip.NewReader(targz)
	if err != nil {
		return fmt.Errorf("%s could not be read: %v", filename, err)
	}

	tarfile := tar.NewReader(gzipReader)
	for {
		header, err := tarfile.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("%s's tar'd files failed to read: %v", filename, err)
		}

		err = fn(tarfile, header)
		if err != nil {
			return err
		}
	}
}
