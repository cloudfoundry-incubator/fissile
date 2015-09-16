package model

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pivotal-golang/archiver/extractor"
)

type Package struct {
	Name         string
	Version      string
	Fingerprint  string
	Sha1         string
	Release      *Release
	Path         string
	Dependencies []*Package

	packageReleaseInfo map[interface{}]interface{}
}

func newPackage(release *Release, packageReleaseInfo map[interface{}]interface{}) (*Package, error) {
	pkg := &Package{
		Release:      release,
		Dependencies: []*Package{},

		packageReleaseInfo: packageReleaseInfo,
	}

	if err := pkg.loadPackageInfo(); err != nil {
		return nil, err
	}

	return pkg, nil
}

// Validates that the SHA1 of the actual package archive is the same
// as the one from the release manifest
func (p *Package) ValidateSha1() error {
	file, err := os.Open(p.Path)
	if err != nil {
		return fmt.Errorf("Error opening the package archive %s for sha1 calculation", p.Path)
	}

	defer file.Close()

	h := sha1.New()

	_, err = io.Copy(h, file)
	if err != nil {
		return fmt.Errorf("Error copying package archive %s for sha1 calculation", p.Path)
	}

	computedSha1 := fmt.Sprintf("%x", h.Sum(nil))

	if computedSha1 != p.Sha1 {
		return fmt.Errorf("Computed sha1 (%s) is different than manifest sha1 (%s) for package archive %s", computedSha1, p.Sha1, p.Path)
	}

	return nil
}

// Extracts the contents of the package archive to destination
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

// Creates a directory structure that contains the package
// and all its dependencies, suitable for executing the compilation package
func (p *Package) PrepareForCompilation(destrination string) error {
	return fmt.Errorf("TODO: Not implemented")
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
	p.Sha1 = p.packageReleaseInfo["sha1"].(string)
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
		dependency, err := p.Release.lookupPackage(pkgName)
		if err != nil {
			return fmt.Errorf("Cannot find dependency for package %s: %v", p.Name, err.Error())
		}

		p.Dependencies = append(p.Dependencies, dependency)
	}

	return nil
}

func (p *Package) packageArchivePath() string {
	return fmt.Sprintf("%s.tgz", filepath.Join(p.Release.packagesDirPath(), p.Name))
}
