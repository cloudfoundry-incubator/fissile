package model

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/hpcloud/fissile/util"

	"github.com/cppforlife/go-semi-semantic/version"
	"gopkg.in/yaml.v2"
)

// NewRelease will create an instance of a BOSH development release
func NewDevRelease(path, releaseName, version, boshCacheDir string) (*Release, error) {
	release := &Release{
		Path:            path,
		Packages:        []*Package{},
		Jobs:            []*Job{},
		Dev:             true,
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

	return release, nil
}

func (r *Release) getDefaultDevReleaseName() (ver string, err error) {
	defer func() {
		if re := recover(); re != nil {
			err = fmt.Errorf("Error trying to load dev configuration file for release %s: %s", r.Name, re)
		}
	}()

	devReleaseConfigContent, err := ioutil.ReadFile(r.getDevReleaseDevConfigFile())
	if err != nil {
		return "", err
	}

	var devReleaseConfig map[interface{}]interface{}

	if err := yaml.Unmarshal([]byte(devReleaseConfigContent), &devReleaseConfig); err != nil {
		return "", err
	}

	return devReleaseConfig["dev_name"].(string), nil
}

func (r *Release) getLatestDevVersion() (ver string, err error) {
	defer func() {
		if re := recover(); re != nil {
			err = fmt.Errorf("Error trying to load dev release index for release %s: %s", r.Name, re)
		}
	}()

	devReleaseIndexContent, err := ioutil.ReadFile(r.getDevReleaseIndexPath())
	if err != nil {
		return "", err
	}

	var devReleaseIndex map[interface{}]interface{}

	if err := yaml.Unmarshal([]byte(devReleaseIndexContent), &devReleaseIndex); err != nil {
		return "", err
	}

	var semiVer version.Version

	for _, build := range devReleaseIndex["builds"].(map[interface{}]interface{}) {
		buildVersion := build.(map[interface{}]interface{})["version"].(string)

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

func (r *Release) getDevReleaseDevConfigFile() string {
	return filepath.Join(r.getDevReleaseConfigDir(), "dev.yml")
}
