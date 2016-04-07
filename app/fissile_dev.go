package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hpcloud/fissile/builder"
	"github.com/hpcloud/fissile/compilator"
	"github.com/hpcloud/fissile/config-store"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"

	"github.com/fatih/color"
)

// ListDevPackages will list all BOSH packages within a list of dev releases
func (f *Fissile) ListDevPackages() error {
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
func (f *Fissile) ListDevJobs() error {
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

// CompileDev will compile a list of dev BOSH releases
func (f *Fissile) CompileDev(repository, targetPath string, workerCount int) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loadedf")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	for _, release := range f.releases {
		f.ui.Println(color.GreenString("Compiling packages for dev release %s (%s) ...", color.YellowString(release.Name), color.MagentaString(release.Version)))

		comp, err := compilator.NewCompilator(dockerManager, targetPath, repository, compilation.UbuntuBase, f.Version, false, f.ui)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %s", err.Error())
		}

		if err := comp.Compile(workerCount, release); err != nil {
			return fmt.Errorf("Error compiling packages: %s", err.Error())
		}
	}

	return nil
}

// GenerateRoleDevImages generates all role images using dev releases
func (f *Fissile) GenerateRoleDevImages(targetPath, repository string, noBuild, force bool, rolesManifestPath, compiledPackagesPath, lightManifestPath, darkManifestPath string) error {
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

// ListDevRoleImages lists all dev role images
func (f *Fissile) ListDevRoleImages(repository string, rolesManifestPath string, existingOnDocker, withVirtualSize bool) error {
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

//GenerateDevConfigurationBase generates a configuration base using dev BOSH releases and opinions from manifests
func (f *Fissile) GenerateDevConfigurationBase(rolesManifestPath, lightManifestPath, darkManifestPath, targetPath, provider string) error {
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

func (f *Fissile) loadDevReleases(releasePaths, releaseNames, releaseVersions []string, cacheDir string) error {
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

// DiffDevConfigurationBases generates a diff comparing the specs for two different BOSH releases
func (f *Fissile) DiffDevConfigurationBases(releasePaths []string, cacheDir string) error {
	hashDiffs, err := f.GetDiffDevConfigurationBases(releasePaths, cacheDir)
	if err != nil {
		return err
	}
	f.reportHashDiffs(hashDiffs)
	return nil
}

// GetDiffDevConfigurationBases calcs the difference in configs and returns a hash
func (f *Fissile) GetDiffDevConfigurationBases(releasePaths []string, cacheDir string) (*HashDiffs, error) {
	if len(releasePaths) != 2 {
		return nil, fmt.Errorf("expected two release paths, got %d", len(releasePaths))
	}
	defaultValues := []string{}
	err := f.loadDevReleases(releasePaths, defaultValues, defaultValues, cacheDir)
	if err != nil {
		return nil, fmt.Errorf("dev config diff: error loading release information: %s", err)
	}
	return getDiffsFromReleases(f.releases)
}
