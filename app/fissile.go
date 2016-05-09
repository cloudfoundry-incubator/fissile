package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hpcloud/fissile/builder"
	"github.com/hpcloud/fissile/compilator"
	"github.com/hpcloud/fissile/config-store"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"

	"github.com/fatih/color"
	"github.com/hpcloud/termui"
)

// Fissile represents a fissile application
type Fissile struct {
	Version  string
	ui       *termui.UI
	cmdErr   error
	releases []*model.Release // Only applies for some commands
}

// NewFissileApplication creates a new app.Fissile
func NewFissileApplication(version string, ui *termui.UI) *Fissile {
	return &Fissile{
		Version: version,
		ui:      ui,
	}
}

// ShowBaseImage will show details about the base BOSH image
func (f *Fissile) ShowBaseImage(baseImage, repository string) error {
	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	image, err := dockerManager.FindImage(baseImage)
	if err != nil {
		return fmt.Errorf("Error looking up base image %s: %s", baseImage, err.Error())
	}

	comp, err := compilator.NewCompilator(dockerManager, "", repository, compilation.UbuntuBase, f.Version, false, f.ui)
	if err != nil {
		return fmt.Errorf("Error creating a new compilator: %s", err.Error())
	}

	f.ui.Printf("Image: %s\n", color.GreenString(baseImage))
	f.ui.Printf("ID: %s\n", color.GreenString(image.ID))
	f.ui.Printf("Virtual Size: %sMB\n", color.YellowString("%.2f", float64(image.VirtualSize)/(1024*1024)))

	image, err = dockerManager.FindImage(comp.BaseImageName())
	if err != nil {
		return fmt.Errorf("Error looking up base image %s: %s", baseImage, err.Error())
	}

	f.ui.Printf("Image: %s\n", color.GreenString(comp.BaseImageName()))
	f.ui.Printf("ID: %s\n", color.GreenString(image.ID))
	f.ui.Printf("Virtual Size: %sMB\n", color.YellowString("%.2f", float64(image.VirtualSize)/(1024*1024)))

	return nil
}

// CreateBaseCompilationImage will recompile the base BOSH image for a release
func (f *Fissile) CreateBaseCompilationImage(baseImageName, repository string, keepContainer bool) error {
	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	baseImage, err := dockerManager.FindImage(baseImageName)
	if err != nil {
		return fmt.Errorf("Error looking up base image %s: %s", baseImageName, err)
	}

	f.ui.Println(color.GreenString("Base image with ID %s found", color.YellowString(baseImage.ID)))

	comp, err := compilator.NewCompilator(dockerManager, "", repository, compilation.UbuntuBase, f.Version, keepContainer, f.ui)
	if err != nil {
		return fmt.Errorf("Error creating a new compilator: %s", err.Error())
	}

	if _, err := comp.CreateCompilationBase(baseImageName); err != nil {
		return fmt.Errorf("Error creating compilation base image: %s", err.Error())
	}

	return nil
}

// GenerateBaseDockerImage generates a base docker image to be used as a FROM for role images
func (f *Fissile) GenerateBaseDockerImage(targetPath, configginTarball, baseImage string, noBuild bool, repository string) error {
	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	baseImageName := builder.GetBaseImageName(repository, f.Version)

	image, err := dockerManager.FindImage(baseImageName)
	if err == docker.ErrImageNotFound {
		f.ui.Println("Image doesn't exist, it will be created ...")
	} else if err != nil {
		return fmt.Errorf("Error looking up image: %s", err.Error())
	} else {
		f.ui.Println(color.GreenString(
			"Base role image %s with ID %s already exists. Doing nothing.",
			color.YellowString(baseImageName),
			color.YellowString(image.ID),
		))
		return nil
	}

	if !strings.HasSuffix(targetPath, string(os.PathSeparator)) {
		targetPath = fmt.Sprintf("%s%c", targetPath, os.PathSeparator)
	}

	baseImageBuilder := builder.NewBaseImageBuilder(baseImage)

	f.ui.Println("Creating Dockerfile ...")

	if err := baseImageBuilder.CreateDockerfileDir(targetPath, configginTarball); err != nil {
		return fmt.Errorf("Error creating Dockerfile and/or assets: %s", err.Error())
	}

	f.ui.Println("Dockerfile created.")

	if !noBuild {
		f.ui.Println("Building docker image ...")

		baseImageName := builder.GetBaseImageName(repository, f.Version)
		log := new(bytes.Buffer)
		stdoutWriter := docker.NewFormattingWriter(
			log,
			docker.ColoredBuildStringFunc(baseImageName),
		)

		err = dockerManager.BuildImage(targetPath, baseImageName, stdoutWriter)
		if err != nil {
			log.WriteTo(f.ui)
			return fmt.Errorf("Error building base image: %s", err.Error())
		}

	} else {
		f.ui.Println("Skipping image build because of flag.")
	}

	f.ui.Println(color.GreenString("Done."))

	return nil
}

// ListDevPackages will list all BOSH packages within a list of dev releases
func (f *Fissile) ListPackages() error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.releases {
		f.ui.Println(color.GreenString("Dev release %s (%s)", color.YellowString(release.Name), color.MagentaString(release.Version)))

		for _, pkg := range release.Packages {
			f.ui.Printf("%s (%s)\n", color.YellowString(pkg.Name), color.WhiteString(pkg.Version))
		}

		f.ui.Printf(
			"There are %s packages present.\n\n",
			color.GreenString("%d", len(release.Packages)),
		)
	}

	return nil
}

// ListDevJobs will list all jobs within a list of dev releases
func (f *Fissile) ListJobs() error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.releases {
		f.ui.Println(color.GreenString("Dev release %s (%s)", color.YellowString(release.Name), color.MagentaString(release.Version)))

		for _, job := range release.Jobs {
			f.ui.Printf("%s (%s): %s\n", color.YellowString(job.Name), color.WhiteString(job.Version), job.Description)
		}

		f.ui.Printf(
			"There are %s jobs present.\n\n",
			color.GreenString("%d", len(release.Jobs)),
		)
	}

	return nil
}

// Compile will compile a list of dev BOSH releases
func (f *Fissile) Compile(repository, targetPath, roleManifestPath string, workerCount int) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	for _, release := range f.releases {
		f.ui.Println(color.GreenString("Compiling packages for dev release %s (%s) ...", color.YellowString(release.Name), color.MagentaString(release.Version)))

		comp, err := compilator.NewCompilator(dockerManager, targetPath, repository, compilation.UbuntuBase, f.Version, false, f.ui)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %s", err.Error())
		}

		if err := comp.Compile(workerCount, release, roleManifest); err != nil {
			return fmt.Errorf("Error compiling packages: %s", err.Error())
		}
	}

	return nil
}

// GenerateRoleImages generates all role images using dev releases
func (f *Fissile) GenerateRoleImages(targetPath, repository string, noBuild, force bool, rolesManifestPath, compiledPackagesPath, lightManifestPath, darkManifestPath string) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	roleBuilder := builder.NewRoleImageBuilder(
		repository,
		compiledPackagesPath,
		targetPath,
		"",
		f.Version,
		f.ui,
	)

	// Generate configuration
	configTargetPath, err := ioutil.TempDir("", "role-spec-config")
	if err != nil {
		return fmt.Errorf("Error creating temporary directory %s: %s", configTargetPath, err.Error())
	}

	defer func() {
		os.RemoveAll(configTargetPath)
	}()

	f.ui.Println("Generating configuration JSON specs ...")

	configStore := configstore.NewConfigStoreBuilder(
		configstore.JSONProvider,
		lightManifestPath,
		darkManifestPath,
		configTargetPath,
	)

	if err := configStore.WriteBaseConfig(rolesManifest); err != nil {
		return fmt.Errorf("Error writing base config: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Done."))

	// Go through each role and create its image
	for _, role := range rolesManifest.Roles {
		roleImageName := builder.GetRoleDevImageName(repository, role, role.GetRoleDevVersion())

		_, err = dockerManager.FindImage(roleImageName)
		if err == nil && !force {
			f.ui.Printf("Dev image %s for role %s already exists. Skipping ...\n", roleImageName, color.YellowString(role.Name))
			continue
		}

		// Remove existing Dockerfile directory
		roleDir := filepath.Join(targetPath, role.Name)
		if err := os.RemoveAll(roleDir); err != nil {
			return fmt.Errorf("Error removing Dockerfile directory and/or assets for role %s: %s", role.Name, err.Error())
		}

		f.ui.Printf("Creating Dockerfile for role %s ...\n", color.YellowString(role.Name))
		dockerfileDir, err := roleBuilder.CreateDockerfileDir(role, configTargetPath)
		if err != nil {
			return fmt.Errorf("Error creating Dockerfile and/or assets for role %s: %s", role.Name, err.Error())
		}

		if !noBuild {
			if !strings.HasSuffix(dockerfileDir, string(os.PathSeparator)) {
				dockerfileDir = fmt.Sprintf("%s%c", dockerfileDir, os.PathSeparator)
			}

			f.ui.Printf("Building docker image in %s ...\n", color.YellowString(dockerfileDir))
			log := new(bytes.Buffer)
			stdoutWriter := docker.NewFormattingWriter(
				log,
				docker.ColoredBuildStringFunc(roleImageName),
			)

			err = dockerManager.BuildImage(dockerfileDir, roleImageName, stdoutWriter)
			if err != nil {
				log.WriteTo(f.ui)
				return fmt.Errorf("Error building image: %s", err.Error())
			}

		} else {
			f.ui.Println("Skipping image build because of flag.")
		}
	}

	f.ui.Println(color.GreenString("Done."))

	return nil
}

// ListRoleImages lists all dev role images
func (f *Fissile) ListRoleImages(repository string, rolesManifestPath string, existingOnDocker, withVirtualSize bool) error {
	if withVirtualSize && !existingOnDocker {
		return fmt.Errorf("Cannot list image virtual sizes if not matching image names with docker")
	}

	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	var dockerManager *docker.ImageManager
	var err error

	if existingOnDocker {
		dockerManager, err = docker.NewImageManager()
		if err != nil {
			return fmt.Errorf("Error connecting to docker: %s", err.Error())
		}
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	for _, role := range rolesManifest.Roles {
		imageName := builder.GetRoleDevImageName(repository, role, role.GetRoleDevVersion())

		if !existingOnDocker {
			f.ui.Println(imageName)
			continue
		}

		image, err := dockerManager.FindImage(imageName)

		if err == docker.ErrImageNotFound {
			continue
		} else if err != nil {
			return fmt.Errorf("Error looking up image: %s", err.Error())
		}

		if withVirtualSize {
			f.ui.Printf(
				"%s (%sMB)\n",
				color.GreenString(imageName),
				color.YellowString("%.2f", float64(image.VirtualSize)/(1024*1024)),
			)
		} else {
			f.ui.Println(imageName)
		}
	}

	return nil
}

//GenerateConfigurationBase generates a configuration base using dev BOSH releases and opinions from manifests
func (f *Fissile) GenerateConfigurationBase(rolesManifestPath, lightManifestPath, darkManifestPath, targetPath, provider string) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	configStore := configstore.NewConfigStoreBuilder(provider, lightManifestPath, darkManifestPath, targetPath)

	if err := configStore.WriteBaseConfig(rolesManifest); err != nil {
		return fmt.Errorf("Error writing base config: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Done."))

	return nil
}

//LoadReleases loads information about BOSH releases
func (f *Fissile) LoadReleases(releasePaths, releaseNames, releaseVersions []string, cacheDir string) error {
	releases := make([]*model.Release, len(releasePaths))

	for idx, releasePath := range releasePaths {
		var releaseName, releaseVersion string

		if len(releaseNames) != 0 {
			releaseName = releaseNames[idx]
		}

		if len(releaseVersions) != 0 {
			releaseVersion = releaseVersions[idx]
		}

		release, err := model.NewDevRelease(releasePath, releaseName, releaseVersion, cacheDir)
		if err != nil {
			return fmt.Errorf("Error loading release information: %s", err.Error())
		}

		releases[idx] = release
	}

	f.releases = releases
	return nil
}

// DiffConfigurationBases generates a diff comparing the specs for two different BOSH releases
func (f *Fissile) DiffConfigurationBases(releasePaths []string, cacheDir string) error {
	hashDiffs, err := f.GetDiffConfigurationBases(releasePaths, cacheDir)
	if err != nil {
		return err
	}
	f.reportHashDiffs(hashDiffs)
	return nil
}

// GetDiffConfigurationBases calculates the difference in configs and returns a hash
func (f *Fissile) GetDiffConfigurationBases(releasePaths []string, cacheDir string) (*HashDiffs, error) {
	if len(releasePaths) != 2 {
		return nil, fmt.Errorf("expected two release paths, got %d", len(releasePaths))
	}
	defaultValues := []string{}
	err := f.LoadReleases(releasePaths, defaultValues, defaultValues, cacheDir)
	if err != nil {
		return nil, fmt.Errorf("dev config diff: error loading release information: %s", err)
	}
	return getDiffsFromReleases(f.releases)
}

type keyHash map[string]string

// HashDiffs summarizes the diffs between the two configs
type HashDiffs struct {
	AddedKeys     []string
	DeletedKeys   []string
	ChangedValues map[string][2]string
}

func (f *Fissile) reportHashDiffs(hashDiffs *HashDiffs) {
	if len(hashDiffs.DeletedKeys) > 0 {
		f.ui.Println(color.RedString("Dropped keys:"))
		sort.Strings(hashDiffs.DeletedKeys)
		for _, v := range hashDiffs.DeletedKeys {
			f.ui.Printf("  %s\n", v)
		}
	}
	if len(hashDiffs.AddedKeys) > 0 {
		f.ui.Println(color.GreenString("Added keys:"))
		sort.Strings(hashDiffs.AddedKeys)
		for _, v := range hashDiffs.AddedKeys {
			f.ui.Printf("  %s\n", v)
		}
	}
	if len(hashDiffs.ChangedValues) > 0 {
		f.ui.Println(color.BlueString("Changed values:"))
		sortedKeys := make([]string, len(hashDiffs.ChangedValues))
		i := 0
		for k := range hashDiffs.ChangedValues {
			sortedKeys[i] = k
			i++
		}
		sort.Strings(sortedKeys)
		for _, k := range sortedKeys {
			v := hashDiffs.ChangedValues[k]
			f.ui.Printf("  %s: %s => %s\n", k, v[0], v[1])
		}
	}
}

func getDiffsFromReleases(releases []*model.Release) (*HashDiffs, error) {
	hashes := [2]keyHash{keyHash{}, keyHash{}}
	for idx, release := range releases {
		configs := release.GetUniqueConfigs()
		for _, config := range configs {
			key, err := configstore.BoshKeyToConsulPath(config.Name, configstore.DescriptionsStore)
			if err != nil {
				return nil, fmt.Errorf("Error getting config %s for release %s: %s", config.Name, release.Name, err.Error())
			}
			hashes[idx][key] = config.Description
		}
		// Get the spec configs
		for _, job := range release.Jobs {
			for _, property := range job.Properties {
				key, err := configstore.BoshKeyToConsulPath(fmt.Sprintf("%s.%s.%s", release.Name, job.Name, property.Name), configstore.SpecStore)
				if err != nil {
					return nil, err
				}
				hashes[idx][key] = fmt.Sprintf("%+v", property.Default)
			}
		}
	}
	return compareHashes(hashes[0], hashes[1]), nil
}

func compareHashes(v1Hash, v2Hash keyHash) *HashDiffs {
	changed := map[string][2]string{}
	deleted := []string{}
	added := []string{}

	for k, v := range v1Hash {
		v2, ok := v2Hash[k]
		if !ok {
			deleted = append(deleted, k)
		} else if v != v2 {
			changed[k] = [2]string{v, v2}
		}
	}
	for k := range v2Hash {
		_, ok := v1Hash[k]
		if !ok {
			added = append(added, k)
		}
	}
	return &HashDiffs{AddedKeys: added, DeletedKeys: deleted, ChangedValues: changed}
}
