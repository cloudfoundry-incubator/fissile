package model

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/hpcloud/fissile/util"

	"github.com/cppforlife/go-semi-semantic/version"
	"gopkg.in/yaml.v2"
)

// NewDevRelease will create an instance of a BOSH development release
func NewDevRelease(path, releaseName, version, boshCacheDir string) (*Release, error) {
	release := &Release{
		Path:            path,
		Name:            releaseName,
		Version:         version,
		DevBOSHCacheDir: boshCacheDir,
	}

	if err := release.validateDevPathStructure(); err != nil {
		return nil, err
	}

	if releaseName == "" {
		releaseName, err := release.getDefaultDevReleaseName()
		if err != nil {
			return nil, err
		}

		release.Name = releaseName
	}

	if err := release.validateSpecificDevReleasePathStructure(); err != nil {
		return nil, err
	}

	if version == "" {
		version, err := release.getLatestDevVersion()
		if err != nil {
			return nil, err
		}

		release.Version = version
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

func (r *Release) getDefaultDevReleaseName() (ver string, err error) {
	releaseConfigContent, err := ioutil.ReadFile(r.getDevReleaseDevConfigFile())
	if err != nil {
		return "", err
	}

	var releaseConfig map[interface{}]interface{}

	if err := yaml.Unmarshal([]byte(releaseConfigContent), &releaseConfig); err != nil {
		return "", err
	}

	var name string
	if value, ok := releaseConfig["dev_name"]; ok {
		if name, ok = value.(string); ok {
			return name, nil
		}
	}

	releaseConfigContent, err = ioutil.ReadFile(r.getDevReleaseFinalConfigFile())
	if err != nil {
		return "", err
	}

	if err := yaml.Unmarshal([]byte(releaseConfigContent), &releaseConfig); err != nil {
		return "", err
	}

	if value, ok := releaseConfig["name"]; !ok {
		if value, ok := releaseConfig["final_name"]; !ok {
			return "", fmt.Errorf("name or final_name key did not exist in configuration file for release: %s", r.Path)
		} else if name, ok = value.(string); !ok {
			return "", fmt.Errorf("final_name was not a string in release: %s, type: %T, value: %v", r.Path, value, value)
		}
	} else if name, ok = value.(string); !ok {
		return "", fmt.Errorf("name was not a string in release: %s, type: %T, value: %v", r.Path, value, value)
	}

	return name, nil
}

func (r *Release) getLatestDevVersion() (ver string, err error) {
	devReleaseIndexContent, err := ioutil.ReadFile(r.getDevReleaseIndexPath())
	if err != nil {
		return "", err
	}

	var devReleaseIndex map[interface{}]interface{}

	if err := yaml.Unmarshal([]byte(devReleaseIndexContent), &devReleaseIndex); err != nil {
		return "", err
	}

	var semiVer version.Version
	var builds map[interface{}]interface{}

	if value, ok := devReleaseIndex["builds"]; !ok {
		return "", fmt.Errorf("builds key did not exist in dev releases index file for release: %s", r.Name)
	} else if builds, ok = value.(map[interface{}]interface{}); !ok {
		return "", fmt.Errorf("builds key in dev releases index file was not a map for release: %s, type: %T, value: %v", r.Name, value, value)
	}

	for _, build := range builds {
		var buildVersion string

		if buildMap, ok := build.(map[interface{}]interface{}); !ok {
			return "", fmt.Errorf("build entry was not a map in release: %s, type: %T, value: %v", r.Name, build, build)
		} else if value, ok := buildMap["version"]; !ok {
			return "", fmt.Errorf("version key did not exist in a build entry for release: %s", r.Name)
		} else if buildVersion, ok = value.(string); !ok {
			return "", fmt.Errorf("version was not a string in a build entry for release: %s, type: %T, value: %v", r.Name, value, value)
		}

		if ver == "" {
			ver = buildVersion
			semiVer, err = version.NewVersionFromString(ver)
			if err != nil {
				return "", err
			}

			continue
		}

		semiBuildVer, err := version.NewVersionFromString(buildVersion)
		if err != nil {
			return "", err
		}

		if semiBuildVer.IsGt(semiVer) {
			ver = buildVersion
			semiVer = semiBuildVer
		}
	}

	return ver, nil
}

func (r *Release) validateDevPathStructure() error {
	if err := util.ValidatePath(r.Path, true, "release directory"); err != nil {
		return err
	}

	if err := util.ValidatePath(r.getDevReleasesDir(), true, "release 'dev_releases' directory"); err != nil {
		return err
	}

	if err := util.ValidatePath(r.getDevReleaseConfigDir(), true, "release config directory"); err != nil {
		return err
	}

	if err := util.ValidatePath(r.getDevReleaseDevConfigFile(), false, "release dev config file"); err != nil {
		return err
	}

	return nil
}

func (r *Release) validateSpecificDevReleasePathStructure() error {
	if err := util.ValidatePath(r.getDevReleaseManifestsDir(), true, "release dev manifests directory"); err != nil {
		return err
	}

	if err := util.ValidatePath(r.getDevReleaseIndexPath(), false, "release index file"); err != nil {
		return err
	}

	return nil
}

func (r *Release) getDevReleaseManifestFilename() string {
	return fmt.Sprintf("%s-%s.yml", r.Name, r.Version)
}

func (r *Release) getDevReleaseManifestsDir() string {
	return filepath.Join(r.getDevReleasesDir(), r.Name)
}

func (r *Release) getDevReleaseIndexPath() string {
	return filepath.Join(r.getDevReleaseManifestsDir(), "index.yml")
}

func (r *Release) getDevReleasesDir() string {
	return filepath.Join(r.Path, "dev_releases")
}

func (r *Release) getDevReleaseConfigDir() string {
	return filepath.Join(r.Path, "config")
}

func (r *Release) getDevReleaseFinalConfigFile() string {
	return filepath.Join(r.getDevReleaseConfigDir(), "final.yml")
}

func (r *Release) getDevReleaseDevConfigFile() string {
	return filepath.Join(r.getDevReleaseConfigDir(), "dev.yml")
}
