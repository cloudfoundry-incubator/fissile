package model

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/hpcloud/fissile/util"

	"gopkg.in/yaml.v2"
)

// Release represents a BOSH release
type Release struct {
	Jobs               Jobs
	Packages           Packages
	License            ReleaseLicense
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

// yamlBinaryRegexp is the regexp used to look for the "!binary" YAML tag; see
// loadMetadata() where it is used.
var yamlBinaryRegexp = regexp.MustCompile(`([^!])!binary \|-\n`)

// NewRelease will create an instance of a BOSH release
func NewRelease(path string) (*Release, error) {
	release := &Release{
		Path:    path,
		License: ReleaseLicense{},
		Dev:     false,
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

	if err := release.loadPackageLicenses(); err != nil {
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
					Jobs:        Jobs{job},
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

	// Psych (the Ruby YAML serializer) will incorrectly emit "!binary" when it means "!!binary".
	// This causes the data to be read incorrectly (not base64 decoded), which causes integrity checks to fail.
	// See https://github.com/tenderlove/psych/blob/c1decb1fef5/lib/psych/visitors/yaml_tree.rb#L309
	manifestContents = yamlBinaryRegexp.ReplaceAll(
		manifestContents,
		[]byte("$1!!binary |-\n"),
	)

	err = yaml.Unmarshal([]byte(manifestContents), &r.manifest)
	if err != nil {
		return err
	}

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

func (r *Release) loadPackageLicenses() error {
	for _, pkg := range r.Packages {
		if err := pkg.loadLicenseFiles(); err != nil {
			return fmt.Errorf("package [%s] licenses could not be read: %v", pkg.Name, err)
		}
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
	r.License.Files = make(map[string][]byte)

	if licenseInfo, ok := r.manifest["license"].(map[interface{}]interface{}); ok {
		if licenseHash, ok := licenseInfo["sha1"].(string); ok {
			r.License.SHA1 = licenseHash
		}
	}

	targz, err := os.Open(r.licenseArchivePath())
	if os.IsNotExist(err) {
		if r.License.SHA1 == "" {
			// There were never licenses to load.
			return nil
		}

		// If we get here, we've encountered a bosh-created release archive.
		// It has a checksum for licenses.tgz, but the actual tgz is missing
		// (instead, bosh placed all the files _inside_ the license archive in
		// the top level of the release archive).  We can't do anything with the
		// checksum, but we can still put the license files into the release.
		// It is unclear why (some of) the releases downloaded from bosh.io do
		// contain license.tgz.

		parent, err := os.Open(r.Path)
		if err != nil {
			return err
		}
		defer parent.Close()
		names, err := parent.Readdirnames(0)
		if err != nil {
			return err
		}
		for _, name := range names {
			namePrefix := name[:len(name)-len(path.Ext(name))]
			if namePrefix == "LICENSE" || namePrefix == "NOTICE" {
				buf, err := ioutil.ReadFile(filepath.Join(r.Path, name))
				if err != nil {
					return err
				}
				r.License.Files[name] = buf
			}
		}

		if len(r.License.Files) == 0 {
			// We had an expected license checksum, but missing license.tgz and
			// no extra license files that were just lying around either.
			return fmt.Errorf("License file is missing")
		}

		return nil
	}
	if err != nil {
		return err
	}
	defer targz.Close()

	hash := sha1.New()

	r.License.Files, err = util.LoadLicenseFiles(
		r.licenseArchivePath(),
		io.TeeReader(targz, hash),
		util.DefaultLicensePrefixFilters...,
	)

	if err != nil {
		return err
	}

	r.License.ActualSHA1 = fmt.Sprintf("%02x", hash.Sum(nil))

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
	}

	return filepath.Join(r.Path, manifestFile)
}
