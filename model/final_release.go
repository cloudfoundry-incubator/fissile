package model

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

// NewFinalRelease will create an instance of a BOSH final release
func NewFinalRelease(path string) (release *Release, err error) {
	release = &Release{
		Path:         path,
		Name:         "",
		Version:      "",
		FinalRelease: true,
	}

	release.Name, err = release.getFinalReleaseName()
	if err != nil {
		return nil, err
	}

	release.Version, err = release.getFinalReleaseVersion()
	if err != nil {
		return nil, err
	}

	if err := release.loadMetadata(); err != nil {
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

	if err := release.loadLicense(); err != nil {
		return nil, err
	}

	return release, nil
}

func (r *Release) getFinalReleaseName() (name string, err error) {
	var releaseConfig map[interface{}]interface{}

	releaseConfigContent, err := ioutil.ReadFile(r.getFinalReleaseConfigFile())
	if err != nil {
		return "", err
	}

	if err := yaml.Unmarshal([]byte(releaseConfigContent), &releaseConfig); err != nil {
		return "", err
	}

	if value, ok := releaseConfig["name"]; !ok {
		return "", fmt.Errorf("name does not exist in release.MF file for release: %s", r.Path)
	} else if name, ok = value.(string); !ok {
		return "", fmt.Errorf("name was not a string in release.MF: %s, type: %T, value: %v", r.Path, value, value)
	}

	return name, nil
}

func (r *Release) getFinalReleaseVersion() (version string, err error) {
	var releaseConfig map[interface{}]interface{}

	releaseConfigContent, err := ioutil.ReadFile(r.getFinalReleaseConfigFile())
	if err != nil {
		return "", err
	}

	if err := yaml.Unmarshal([]byte(releaseConfigContent), &releaseConfig); err != nil {
		return "", err
	}

	if value, ok := releaseConfig["version"]; !ok {
		return "", fmt.Errorf("version does not exist in release.MF file for release: %s", r.Name)
	} else if version, ok = value.(string); !ok {
		return "", fmt.Errorf("version is not a string in release.MF: %s, type: %T, value: %v", r.Name, value, value)
	}

	return version, nil
}

func (r *Release) getFinalReleaseManifestFilename() string {
	return "release.MF"
}

func (r *Release) getFinalReleaseConfigFile() string {
	return filepath.Join(r.Path, "release.MF")
}
