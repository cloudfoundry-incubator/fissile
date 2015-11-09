package model

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pivotal-golang/archiver/extractor"
)

// Package represents a BOSH package
type Package struct {
	Name         string
	Version      string
	Fingerprint  string
	SHA1         string
	Release      *Release
	Path         string
	Dependencies Packages

	packageReleaseInfo map[interface{}]interface{}
}

// Packages is an array of *Package
type Packages []*Package

func newPackage(release *Release, packageReleaseInfo map[interface{}]interface{}) (*Package, error) {
	pkg := &Package{
		Release:      release,
		Dependencies: Packages{},

		packageReleaseInfo: packageReleaseInfo,
	}

	if err := pkg.loadPackageInfo(); err != nil {
		return nil, err
	}

	return pkg, nil
}

// ValidateSHA1 validates that the SHA1 of the actual package archive is the same
// as the one from the release manifest
func (p *Package) ValidateSHA1() error {
	file, err := os.Open(p.Path)
	if err != nil {
		return fmt.Errorf("Error opening the package archive %s for SHA1 calculation", p.Path)
	}

	defer file.Close()

	h := sha1.New()

	_, err = io.Copy(h, file)
	if err != nil {
		return fmt.Errorf("Error copying package archive %s for SHA1 calculation", p.Path)
	}

	computedSHA1 := fmt.Sprintf("%x", h.Sum(nil))

	if computedSHA1 != p.SHA1 {
		return fmt.Errorf("Computed SHA1 (%s) is different than manifest SHA1 (%s) for package archive %s", computedSHA1, p.SHA1, p.Path)
	}

	return nil
}

// Extract will extract the contents of the package archive to destination
// It creates a directory with the name of the package
// Returns the full path of the extracted archive
func (p *Package) Extract(destination string) (string, error) {
	targetDir := filepath.Join(destination, p.Name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	if err := extractor.NewTgz().Extract(p.Path, targetDir); err != nil {
		return "", err
	}

	return targetDir, nil
}

// Len implements the Len function to satisfy sort.Interface
func (slice Packages) Len() int {
	return len(slice)
}

// Less implements the Less function to satisfy sort.Interface
func (slice Packages) Less(i, j int) bool {
	return slice[i].Name < slice[j].Name
}

// Swap implements the Swap function to satisfy sort.Interface
func (slice Packages) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (p *Package) loadPackageInfo() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error trying to load package information: %s", r)
		}
	}()

	p.Name = p.packageReleaseInfo["name"].(string)
	p.Version = p.packageReleaseInfo["version"].(string)
	p.Fingerprint = p.packageReleaseInfo["fingerprint"].(string)
	p.SHA1 = p.packageReleaseInfo["sha1"].(string)
	p.Path = p.packageArchivePath()

	return nil
}

func (p *Package) loadPackageDependencies() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error trying to load package information: %s", r)
		}
	}()

	for _, pkg := range p.packageReleaseInfo["dependencies"].([]interface{}) {
		pkgName := pkg.(string)
		dependency, err := p.Release.LookupPackage(pkgName)
		if err != nil {
			return fmt.Errorf("Cannot find dependency for package %s: %v", p.Name, err.Error())
		}

		p.Dependencies = append(p.Dependencies, dependency)
	}

	return nil
}

func (p *Package) packageArchivePath() string {
	if p.Release.Dev {
		return filepath.Join(p.Release.DevBOSHCacheDir, p.SHA1)
	}

	return fmt.Sprintf("%s.tgz", filepath.Join(p.Release.packagesDirPath(), p.Name))
}
