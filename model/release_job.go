package model

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/SUSE/fissile/util"

	"code.cloudfoundry.org/archiver/extractor"
	"gopkg.in/yaml.v2"
)

// jobLinkInfo describes a BOSH link provider or consumer
type jobLinkInfo struct {
	Name     string `json:"-" yaml:"-"`
	Type     string `json:"-" yaml:"-"`
	RoleName string `json:"role" yaml:"-"`
	JobName  string `json:"job" yaml:"-"`
}

// jobProvidesInfo describes a BOSH link provider
type jobProvidesInfo struct {
	jobLinkInfo
	Alias      string `yaml:"as"`
	Shared     bool   `yaml:"shared"`
	Properties []string
}

// jobConsumesInfo describes the BOSH links a job consumes
type jobConsumesInfo struct {
	jobLinkInfo
	Alias    string `yaml:"from"`
	Optional bool
}

// ReleaseJob represents a BOSH job
type ReleaseJob struct {
	Name               string
	Description        string
	Templates          []*JobTemplate
	Packages           Packages
	Path               string
	Fingerprint        string
	SHA1               string
	SpecProperties     []*JobSpecProperty
	Version            string
	Release            *Release
	AvailableProviders map[string]jobProvidesInfo
	DesiredConsumers   []jobConsumesInfo

	jobReleaseInfo map[interface{}]interface{}
}

// ReleaseJobs is an array of Job*
type ReleaseJobs []*ReleaseJob

func newReleaseJob(release *Release, jobReleaseInfo map[interface{}]interface{}) (*ReleaseJob, error) {
	job := &ReleaseJob{
		Release: release,

		jobReleaseInfo: jobReleaseInfo,
	}

	if err := job.loadJobInfo(); err != nil {
		return nil, err
	}

	if err := job.loadJobSpec(); err != nil {
		return nil, err
	}

	return job, nil
}

func (j *ReleaseJob) getProperty(name string) (*JobSpecProperty, error) {
	for _, property := range j.SpecProperties {
		if property.Name == name {
			return property, nil
		}
	}

	return nil, fmt.Errorf("Property %s not found in job %s", name, j.Name)
}

// ValidateSHA1 validates that the SHA1 of the actual job archive is the same
// as the one from the release manifest
func (j *ReleaseJob) ValidateSHA1() error {
	file, err := os.Open(j.Path)
	if err != nil {
		return fmt.Errorf("Error opening the job archive %s for sha1 calculation", j.Path)
	}

	defer file.Close()

	h := sha1.New()

	_, err = io.Copy(h, file)
	if err != nil {
		return fmt.Errorf("Error copying job archive %s for sha1 calculation", j.Path)
	}

	computedSha1 := fmt.Sprintf("%x", h.Sum(nil))

	if computedSha1 != j.SHA1 {
		return fmt.Errorf("Computed sha1 (%s) is different than manifest sha1 (%s) for job archive %s", computedSha1, j.SHA1, j.Path)
	}

	return nil
}

// Extract will extract the contents of the job archive to destination
// It creates a directory with the name of the job
// Returns the full path of the extracted archive
func (j *ReleaseJob) Extract(destination string) (string, error) {
	targetDir := filepath.Join(destination, j.Name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	if err := extractor.NewTgz().Extract(j.Path, targetDir); err != nil {
		return "", err
	}

	return targetDir, nil
}

func (j *ReleaseJob) loadJobInfo() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error trying to load job information: %s", r)
		}
	}()

	j.Name = j.jobReleaseInfo["name"].(string)
	j.Version = j.jobReleaseInfo["version"].(string)
	j.Fingerprint = j.jobReleaseInfo["fingerprint"].(string)
	j.SHA1 = j.jobReleaseInfo["sha1"].(string)
	j.Path = j.jobArchivePath()

	return nil
}

func (j *ReleaseJob) loadJobSpec() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error trying to load job spec: %s", r)
		}
	}()

	tempJobDir, err := ioutil.TempDir("", "fissile-job-dir")
	defer func() {
		if cleanupErr := os.RemoveAll(tempJobDir); cleanupErr != nil && err != nil {
			err = fmt.Errorf("Error loading job spec: %v,  cleanup error: %v", err, cleanupErr)
		} else if cleanupErr != nil {
			err = fmt.Errorf("Error cleaning up after load job spec: %v", cleanupErr)
		}
	}()
	if err != nil {
		return err
	}

	jobDir, err := j.Extract(tempJobDir)
	if err != nil {
		return fmt.Errorf("Error extracting archive (%s) for job %s: %s", j.Path, j.Name, err.Error())
	}

	specFile := filepath.Join(jobDir, "job.MF")

	specContents, err := ioutil.ReadFile(specFile)
	if err != nil {
		return err
	}

	// jobSpec describes the contents of "job.MF" files
	var jobSpec struct {
		Name        string
		Description string
		Packages    []string
		Templates   map[string]string
		Properties  map[string]struct {
			Description string
			Default     interface{}
			Example     interface{}
		}
		Consumes []struct {
			Name     string
			Type     string
			Optional bool
		}
		Provides []struct {
			Name       string
			Type       string
			Properties []string
		}
	}

	if err := yaml.Unmarshal([]byte(specContents), &jobSpec); err != nil {
		return err
	}

	j.Description = jobSpec.Description

	for _, pkgName := range jobSpec.Packages {
		dependency, err := j.Release.LookupPackage(pkgName)
		if err != nil {
			return fmt.Errorf("Cannot find dependency for job %s: %v", j.Name, err.Error())
		}

		j.Packages = append(j.Packages, dependency)
	}

	for source, destination := range jobSpec.Templates {
		templateFile := filepath.Join(jobDir, "templates", source)

		templateContent, err := ioutil.ReadFile(templateFile)
		if err != nil {
			return err
		}

		template := &JobTemplate{
			SourcePath:      source,
			DestinationPath: destination,
			Job:             j,
			Content:         string(templateContent),
		}

		j.Templates = append(j.Templates, template)
	}

	// We want to load the properties in sorted order, so that we are
	// consistent and avoid flaky tests
	var propertyNames []string
	for propertyName := range jobSpec.Properties {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)
	for _, propertyName := range propertyNames {

		property := &JobSpecProperty{
			Name:        propertyName,
			Job:         j,
			Description: jobSpec.Properties[propertyName].Description,
			Default:     jobSpec.Properties[propertyName].Default,
		}

		j.SpecProperties = append(j.SpecProperties, property)
	}

	j.AvailableProviders = make(map[string]jobProvidesInfo)
	for _, provides := range jobSpec.Provides {
		if provides.Type == "" {
			return fmt.Errorf("job %s provider %s has no type", j.Name, provides.Name)
		}
		j.AvailableProviders[provides.Name] = jobProvidesInfo{
			jobLinkInfo: jobLinkInfo{
				Name:    provides.Name,
				Type:    provides.Type,
				JobName: j.Name,
			},
			Properties: provides.Properties,
		}
	}

	j.DesiredConsumers = make([]jobConsumesInfo, 0, len(jobSpec.Consumes))
	for _, consumes := range jobSpec.Consumes {
		if consumes.Type == "" {
			return fmt.Errorf("job %s consumer %s has no type", j.Name, consumes.Name)
		}
		j.DesiredConsumers = append(j.DesiredConsumers, jobConsumesInfo{
			jobLinkInfo: jobLinkInfo{
				Name: consumes.Name,
				Type: consumes.Type,
			},
			Optional: consumes.Optional,
		})
	}

	return nil
}

// MergeSpec is used to merge temporary spec patches into each job. otherJob should only be
// the fissile-compat/patch-properties job.  The code assumes package and property objects are immutable,
// as they're now being shared across jobs. Also, when specified packages or properties are
// specified in the "other" job, that one takes precedence.
func (j *ReleaseJob) MergeSpec(otherJob *ReleaseJob) {
	// Ignore otherJob.Name, otherJob.Description
	// Skip templates -- they're only in place to keep `create-release` happy.
	j.Packages = append(j.Packages, otherJob.Packages...)
	j.SpecProperties = append(j.SpecProperties, otherJob.SpecProperties...)
}

// jobConfigTemplate is one "templates:" entry in the job config output
type jobConfigTemplate struct {
	Name string `json:"name"`
}

// GetPropertiesForJob returns the parameters for the given job, using its specs and opinions
func (j *ReleaseJob) GetPropertiesForJob(opinions *Opinions) (map[string]interface{}, error) {
	props := make(map[string]interface{})
	lightOpinions, ok := opinions.Light["properties"]
	if !ok {
		return nil, fmt.Errorf("getPropertiesForJob: no 'properties' key in light opinions")
	}
	darkOpinions, ok := opinions.Dark["properties"]
	if !ok {
		return nil, fmt.Errorf("getPropertiesForJob: no 'properties' key in dark opinions")
	}
	lightOpinionsByString, ok := lightOpinions.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("getPropertiesForJob: can't convert lightOpinions into a string map")
	}
	darkOpinionsByString, ok := darkOpinions.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("getPropertiesForJob: can't convert darkOpinions into a string map")
	}
	for _, property := range j.SpecProperties {
		keyPieces, err := getKeyGrams(property.Name)
		if err != nil {
			return nil, err
		}

		// The check for darkness does not only test if the
		// presented key is found in the dark opionions, but
		// also the type of the associated value. Excluding a
		// key like "a.b.c.d" does not mean that "a.b.c",
		// etc. are excluded as well. Definitely not. So,
		// finding a key we consider it to be an excluded leaf
		// key only when the associated value, if any is
		// neither map nor array. When finding a map or array,
		// or no value at all we consider the key to be an
		// inner node which is not excluded.

		darkValue, ok := getOpinionValue(darkOpinionsByString, keyPieces)
		if ok {
			if darkValue == nil {
				// Ignore dark opinions
				continue
			}
			kind := reflect.TypeOf(darkValue).Kind()
			if kind != reflect.Map && kind != reflect.Array {
				// Ignore dark opinions
				continue
			}
		}
		lightValue, hasLightValue := getOpinionValue(lightOpinionsByString, keyPieces)
		var finalValue interface{}
		if hasLightValue && lightValue != nil {
			finalValue = lightValue
		} else {
			finalValue = property.Default
		}
		if err := insertConfig(props, property.Name, finalValue); err != nil {
			return nil, err
		}
	}
	return props, nil
}

// Len implements the Len function to satisfy sort.Interface
func (slice ReleaseJobs) Len() int {
	return len(slice)
}

// Less implements the Less function to satisfy sort.Interface
func (slice ReleaseJobs) Less(i, j int) bool {
	return slice[i].Name < slice[j].Name
}

// Swap implements the Swap function to satisfy sort.Interface
func (slice ReleaseJobs) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (j *ReleaseJob) jobArchivePath() string {
	if j.Release.FinalRelease {
		return filepath.Join(j.Release.Path, "jobs", j.Name+".tgz")
	}

	return filepath.Join(j.Release.DevBOSHCacheDir, j.SHA1)
}

// Marshal implements the util.Marshaler interface
func (j *ReleaseJob) Marshal() (interface{}, error) {
	var releaseName string
	if j.Release != nil {
		releaseName = j.Release.Name
	}

	templates := make([]interface{}, 0, len(j.Templates))
	for _, template := range j.Templates {
		templates = append(templates, util.NewMarshalAdapter(template))
	}

	pkgs := make([]string, 0, len(j.Packages))
	for _, pkg := range j.Packages {
		pkgs = append(pkgs, pkg.Fingerprint)
	}

	properties := make([]*JobSpecProperty, 0, len(j.SpecProperties))
	for _, prop := range j.SpecProperties {
		properties = append(properties, prop)
	}

	return map[string]interface{}{
		"name":        j.Name,
		"description": j.Description,
		"templates":   templates,
		"packages":    pkgs,
		"path":        j.Path,
		"fingerprint": j.Fingerprint,
		"sha1":        j.SHA1,
		"properties":  properties,
		"version":     j.Version,
		"release":     releaseName,
	}, nil
}
