package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/SUSE/fissile/util"

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
	DevBOSHCacheDir    string

	manifest map[interface{}]interface{}
}

const (
	jobsDir      = "jobs"
	packagesDir  = "packages"
	manifestFile = "release.MF"
)

// yamlBinaryRegexp is the regexp used to look for the "!binary" YAML tag; see
// loadMetadata() where it is used.
var yamlBinaryRegexp = regexp.MustCompile(`([^!])!binary \|-\n`)

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
		if p := recover(); p != nil {
			err = fmt.Errorf("Error trying to load release metadata from YAML manifest %s: %s", r.manifestFilePath(), p)
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

	licenseFile, err := os.Open(r.licensePath())
	if os.IsNotExist(err) {
		// There were never licenses to load.
		return nil
	}
	if err != nil {
		return err
	}
	defer licenseFile.Close()

	licenseContents, err := ioutil.ReadFile(r.licensePath())
	if err != nil {
		return err
	}

	licenseFilePath, err := filepath.Rel(r.Path, r.licensePath())
	if err != nil {
		return err
	}

	r.License.Files[licenseFilePath] = licenseContents

	return nil
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

func (r *Release) licensePath() string {
	return filepath.Join(r.Path, "LICENSE")
}

func (r *Release) packagesDirPath() string {
	return filepath.Join(r.Path, packagesDir)
}

func (r *Release) jobsDirPath() string {
	return filepath.Join(r.Path, jobsDir)
}

func (r *Release) manifestFilePath() string {
	return filepath.Join(r.getDevReleaseManifestsDir(), r.getDevReleaseManifestFilename())
}

// Marshal implements the util.Marshaler interface
func (r *Release) Marshal() (interface{}, error) {
	jobFingerprints := make([]string, 0, len(r.Jobs))
	for _, job := range r.Jobs {
		jobFingerprints = append(jobFingerprints, job.Fingerprint)
	}

	pkgs := make([]string, 0, len(r.Packages))
	for _, pkg := range r.Packages {
		pkgs = append(pkgs, pkg.Fingerprint)
	}

	licenses := make(map[string]string)
	for name, value := range r.License.Files {
		licenses[name] = string(value)
	}

	return map[string]interface{}{
		"jobs":               jobFingerprints,
		"packages":           pkgs,
		"license":            licenses,
		"name":               r.Name,
		"uncommittedChanges": r.UncommittedChanges,
		"commitHash":         r.CommitHash,
		"version":            r.Version,
		"path":               r.Path,
		"devBOSHCacheDir":    r.DevBOSHCacheDir,
	}, nil
}
