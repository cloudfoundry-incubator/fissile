package compilator

import (
	"io/ioutil"
	"math"

	"github.com/SUSE/fissile/model"

	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/graymeta/stow"
	"github.com/mholt/archiver"
	"github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
	// support Azure storage
	_ "github.com/graymeta/stow/azure"
	// support Google storage
	_ "github.com/graymeta/stow/google"
	// support local storage
	"github.com/graymeta/stow/local"
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
	ReadOnly           bool
}

type packageStorageConfig struct {
	Kind          string `yaml:"boshPackageCacheKind"`
	ReadOnly      bool   `yaml:"boshPackageCacheReadOnly"`
	ContainerPath string `yaml:"boshPackageCacheLocation"`
}

func NewPackageStorageFromConfig(configFilePath, compilationWorkDir, stemcellImageName string) (*PackageStorage, error) {
	// Read a yaml file that contains a stow configuration
	if _, err := os.Stat(configFilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	packageCacheConfigReader, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the package cache config file: %s", err.Error())
	}

	var stowConfig map[string]interface{}
	var packageCacheConfig packageStorageConfig

	if err := yaml.Unmarshal(packageCacheConfigReader, &stowConfig); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal the package cache config file: %s", err.Error())
	}
	if err := yaml.Unmarshal(packageCacheConfigReader, &packageCacheConfig); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal the package cache config file: %s", err.Error())
	}

	var configMap stow.ConfigMap
	configMap = make(stow.ConfigMap)

	for key, value := range stowConfig {
		if key != "boshPackageCacheKind" && key != "boshPackageCacheReadOnly" && key != "boshPackageCacheLocation" {
			configMap.Set(key, value.(string))
		}
	}

	// Generate a new instance of PackageStorage with the data from the config file
	return NewPackageStorage(
		packageCacheConfig.Kind,
		packageCacheConfig.ReadOnly,
		configMap,
		compilationWorkDir,
		packageCacheConfig.ContainerPath,
		stemcellImageName,
	)
}

// NewPackageStorage creates a new PackageStorage instance
func NewPackageStorage(kind string, readOnlyMode bool, config stow.Config, compilationWorkDir string, containerPath string, stemcellImageName string) (p *PackageStorage, err error) {
	stowLocation, err := stow.Dial(kind, config)
	if err != nil {
		return nil, err
	}

	if kind == local.Kind {
		localPath, _ := config.Config(local.ConfigKeyPath)
		fullContainerPath := filepath.Join(localPath, containerPath)
		err = os.MkdirAll(fullContainerPath, 0700)
		if err != nil {
			return nil, err
		}
		containerPath = fullContainerPath
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
		ReadOnly:           readOnlyMode,
	}
	return p, nil
}

// Exists checks whether a package already exists in the configured
// stow cache
func (p *PackageStorage) Exists(pack *model.Package) (bool, error) {
	items, _, err := p.container.Items(p.uploadedPackageFilePath(pack), "", math.MaxInt32)

	if err != nil {
		return false, err
	}

	return len(items) == 1, nil
}

// Download downloads a package from the configured cache
func (p *PackageStorage) Download(pack *model.Package) error {

	// Find the item in the cache
	item, _, err := p.container.Items(p.uploadedPackageFilePath(pack), "", math.MaxInt32)
	cachedPackageReader, err := item[0].Open()
	if err != nil {
		return err
	}
	defer cachedPackageReader.Close()

	// Create a temporary file where to download the package
	path := p.localPackageTempArchivePath(pack)
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	// Cleanup when done
	defer func() {
		file.Close()
		os.RemoveAll(path)
	}()

	// Download the package from the cache
	_, err = io.Copy(file, cachedPackageReader)
	if err != nil {
		return err
	}

	// Unpack the compiled contents
	err = archiver.TarGz.Open(
		path,
		filepath.Join(p.CompilationWorkDir, pack.Fingerprint),
	)

	return nil
}

// Upload uploads a package to the configured cache
func (p *PackageStorage) Upload(pack *model.Package) error {

	// Create a temporary archive with the compiled contents
	archiveName := p.localPackageTempArchivePath(pack)

	// Archive (tgz) the contents
	err := archiver.TarGz.Make(archiveName, []string{pack.GetPackageCompiledDir(p.CompilationWorkDir)})
	// Cleanup the archive when done
	defer os.RemoveAll(archiveName)

	// Get the size of the archive
	fileInfo, err := os.Stat(archiveName)
	if err != nil {
		return err
	}
	file, err := os.Open(archiveName)
	if err != nil {
		return err
	}

	// Upload the compiled package
	_, err = p.container.Put(
		p.uploadedPackageFilePath(pack),
		file,
		fileInfo.Size(),
		nil,
	)

	return err
}

func (p *PackageStorage) uploadedPackageFilePath(pack *model.Package) string {
	return filepath.Join(p.ImageName, p.uploadedPackageFileName(pack))
}

func (p *PackageStorage) uploadedPackageFileName(pack *model.Package) string {
	return fmt.Sprintf("%s.tgz", pack.Fingerprint)
}

func (p *PackageStorage) localPackageTempArchivePath(pack *model.Package) string {
	id := uuid.Must(uuid.NewV4())
	return filepath.Join(os.TempDir(), fmt.Sprintf("package-%s.tgz", id.String()))
}
