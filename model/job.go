package model

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/SUSE/fissile/util"

	"github.com/pivotal-golang/archiver/extractor"
	"gopkg.in/yaml.v2"
)

// Job represents a BOSH job
type Job struct {
	Name        string
	Description string
	Templates   []*JobTemplate
	Packages    Packages
	Path        string
	Fingerprint string
	SHA1        string
	Properties  []*JobProperty
	Version     string
	Release     *Release

	jobReleaseInfo map[interface{}]interface{}
	jobSpec        map[interface{}]interface{}
}

// Jobs is an array of Job*
type Jobs []*Job

func newJob(release *Release, jobReleaseInfo map[interface{}]interface{}) (*Job, error) {
	job := &Job{
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

func (j *Job) getProperty(name string) (*JobProperty, error) {
	for _, property := range j.Properties {
		if property.Name == name {
			return property, nil
		}
	}

	return nil, fmt.Errorf("Property %s not found in job %s", name, j.Name)
}

// ValidateSHA1 validates that the SHA1 of the actual job archive is the same
// as the one from the release manifest
func (j *Job) ValidateSHA1() error {
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
func (j *Job) Extract(destination string) (string, error) {
	targetDir := filepath.Join(destination, j.Name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	if err := extractor.NewTgz().Extract(j.Path, targetDir); err != nil {
		return "", err
	}

	return targetDir, nil
}

func (j *Job) loadJobInfo() (err error) {
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

func (j *Job) loadJobSpec() (err error) {
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

	if err := yaml.Unmarshal([]byte(specContents), &j.jobSpec); err != nil {
		return err
	}

	if j.jobSpec["description"] != nil {
		j.Description = j.jobSpec["description"].(string)
	}

	if j.jobSpec["packages"] != nil {
		for _, pkg := range j.jobSpec["packages"].([]interface{}) {
			pkgName := pkg.(string)
			dependency, err := j.Release.LookupPackage(pkgName)
			if err != nil {
				return fmt.Errorf("Cannot find dependency for job %s: %v", j.Name, err.Error())
			}

			j.Packages = append(j.Packages, dependency)
		}
	}

	if j.jobSpec["templates"] != nil {
		for source, destination := range j.jobSpec["templates"].(map[interface{}]interface{}) {
			templateFile := filepath.Join(jobDir, "templates", source.(string))

			templateContent, err := ioutil.ReadFile(templateFile)
			if err != nil {
				return err
			}

			template := &JobTemplate{
				SourcePath:      source.(string),
				DestinationPath: destination.(string),
				Job:             j,
				Content:         string(templateContent),
			}

			j.Templates = append(j.Templates, template)
		}
	}

	if j.jobSpec["properties"] != nil {
		// We want to load the properties in sorted order, so that we are
		// consistent and avoid flaky tests
		properties := j.jobSpec["properties"].(map[interface{}]interface{})
		var propertyNames []string
		for propertyName := range properties {
			propertyNames = append(propertyNames, propertyName.(string))
		}
		sort.Strings(propertyNames)
		for _, propertyName := range propertyNames {

			property := &JobProperty{
				Name: propertyName,
				Job:  j,
			}

			propertyDefinition := properties[propertyName]
			if propertyDefinition != nil {
				propertyDefinitionMap := propertyDefinition.(map[interface{}]interface{})

				if propertyDefinitionMap["description"] != nil {
					property.Description = propertyDefinitionMap["description"].(string)
				}
				if propertyDefinitionMap["default"] != nil {
					property.Default = propertyDefinitionMap["default"]
				}
			}

			j.Properties = append(j.Properties, property)
		}
	}

	return nil
}

// MergeSpec is used to merge temporary spec patches into each job. otherJob should only be
// the fissile-compat/patch-properties job.  The code assumes package and property objects are immutable,
// as they're now being shared across jobs. Also, when specified packages or properties are
// specified in the "other" job, that one takes precedence.
func (j *Job) MergeSpec(otherJob *Job) {
	// Ignore otherJob.Name, otherJob.Description
	// Skip templates -- they're only in place to keep `create-release` happy.
	j.Packages = append(j.Packages, otherJob.Packages...)
	j.Properties = append(j.Properties, otherJob.Properties...)
}

// WriteConfigs merges the job's spec with the opinions and returns the result as JSON.
func (j *Job) WriteConfigs(role *Role, lightOpinionsPath, darkOpinionsPath string) ([]byte, error) {
	config, err := initializeConfigJSON()
	if err != nil {
		return nil, err
	}

	config["job"].(map[string]interface{})["name"] = role.Name

	var templates []map[string]string
	for _, roleJob := range role.Jobs {
		templates = append(templates, map[string]string{"name": roleJob.Name})
	}
	config["job"].(map[string]interface{})["templates"] = templates

	opinions, err := NewOpinions(lightOpinionsPath, darkOpinionsPath)
	if err != nil {
		return nil, err
	}
	properties, err := j.GetPropertiesForJob(opinions)
	if err != nil {
		return nil, err
	}
	config["properties"] = properties

	// Write out the configuration
	jobJSON, err := json.MarshalIndent(config, "", "    ") // 4-space indent
	if err != nil {
		return nil, err
	}

	return jobJSON, nil
}

// GetPropertiesForJob returns the parameters for the given job, using its specs and opinions
func (j *Job) GetPropertiesForJob(opinions *Opinions) (map[string]interface{}, error) {
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
	for _, property := range j.Properties {
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

// initializeConfigJSON returns the scaffolding for the BOSH-style JSON structure
func initializeConfigJSON() (map[string]interface{}, error) {
	var config map[string]interface{}
	err := json.Unmarshal([]byte(`{
		"job": {
			"templates": []
		},
		"parameters": {},
		"properties": {},
		"networks": {
			"default": {}
		}
	}`), &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal initial config: %+v", err)
	}
	return config, nil
}

// Len implements the Len function to satisfy sort.Interface
func (slice Jobs) Len() int {
	return len(slice)
}

// Less implements the Less function to satisfy sort.Interface
func (slice Jobs) Less(i, j int) bool {
	return slice[i].Name < slice[j].Name
}

// Swap implements the Swap function to satisfy sort.Interface
func (slice Jobs) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (j *Job) jobArchivePath() string {
	return filepath.Join(j.Release.DevBOSHCacheDir, j.SHA1)
}

// Marshal implements the util.Marshaler interface
func (j *Job) Marshal() (interface{}, error) {
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

	properties := make([]*JobProperty, 0, len(j.Properties))
	for _, prop := range j.Properties {
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
