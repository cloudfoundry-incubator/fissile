package compilator

import (
	"github.com/SUSE/fissile/model"

	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/graymeta/stow"
	"github.com/mholt/archiver"
	"github.com/satori/go.uuid"
	// support Azure storage
	_ "github.com/graymeta/stow/azure"
	// support Google storage
	_ "github.com/graymeta/stow/google"
	// support local storage
	_ "github.com/graymeta/stow/local"
	// support swift storage
	_ "github.com/graymeta/stow/swift"
	// support s3 storage
	_ "github.com/graymeta/stow/s3"
	// support oracle storage
	_ "github.com/graymeta/stow/oracle"
)

// PackageStorage represents a compiled BOSH package location
type PackageStorage struct {
	location           stow.Location
	Kind               string
	Config             stow.Config
	CompilationWorkDir string
	container          stow.Container
	ImageName          string
}

// NewPackageStorage creates a new PackageStorage instance
func NewPackageStorage(kind string, config stow.Config, compilationWorkDir string, containerPath string, stemcellImageName string) (p *PackageStorage, err error) {
	stowLocation, err := stow.Dial(kind, config)
	if err != nil {
		return nil, err
	}
	stowContainer, err := stowLocation.Container(containerPath)
	if err != nil {
		return nil, err
	}
	p = &PackageStorage{
		Kind:               kind,
		Config:             config,
		location:           stowLocation,
		CompilationWorkDir: compilationWorkDir,
		container:          stowContainer,
		ImageName:          stemcellImageName,
	}
	return p, nil
}

// Exists checks whether a package already exists in the configured
// stow cache
func (p *PackageStorage) Exists(pack *model.Package) (bool, error) {
	item, _, err := p.container.Items(p.ImageName+pack.Fingerprint+".tgz", "", 1)
	if err != nil {
		return false, err
	}
	if len(item) == 0 {
		return false, nil
	}

	name := item[0].Name()
	var fingerprint string
	if p.Kind == "local" {
		fingerprint = pack.Fingerprint + ".tgz"
	} else {
		if p.Kind == "s3" {
			fingerprint = p.ImageName + pack.Fingerprint + ".tgz"
		}
	}
	if name == fingerprint {
		return true, nil
	}
	return false, nil
}

// Download downloads a package from the configured cache
func (p *PackageStorage) Download(pack *model.Package) error {

	packageCompiledDir := filepath.Join(p.CompilationWorkDir, pack.Fingerprint, "compiled")
	item, _, err := p.container.Items(p.ImageName+pack.Fingerprint+".tgz", "", 1)
	r, err := item[0].Open()
	if err != nil {
		return err
	}
	defer r.Close()

	path := filepath.Join(os.TempDir(), pack.Fingerprint+".tgz")
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer os.RemoveAll(path)
	_, err = io.Copy(file, r)
	if err != nil {
		return err
	}

	err = archiver.TarGz.Open(path, packageCompiledDir)

	file.Close()

	return nil
}

// Upload uploads a package to the configured cache
func (p *PackageStorage) Upload(pack *model.Package) error {

	id := uuid.Must(uuid.NewV4())
	archiveName := filepath.Join(os.TempDir(), fmt.Sprintf("package-%d.tgz", id.String()))
	// Archive (tgz)
	packageCompiledDir := filepath.Join(p.CompilationWorkDir, pack.Fingerprint, "compiled")
	err := archiver.TarGz.Make(archiveName, []string{packageCompiledDir})
	defer os.RemoveAll(archiveName)

	fileInfo, err := os.Stat(archiveName)
	if err != nil {
		return err
	}
	file, err := os.Open(archiveName)
	if err != nil {
		return err
	}

	name := filepath.Join(p.ImageName, pack.Fingerprint+".tgz")

	_, err = p.container.Put(
		name,
		file,
		fileInfo.Size(),
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}
