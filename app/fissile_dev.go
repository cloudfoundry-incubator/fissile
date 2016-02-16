package app

import (
	"bytes"
	"fmt"
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
func (f *Fissile) ListDevPackages(releasePaths, releaseNames, releaseVersions []string, cacheDir string) error {

	releases, err := loadDevReleases(releasePaths, releaseNames, releaseVersions, cacheDir)
	if err != nil {
		return err
	}

	for _, release := range releases {
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
func (f *Fissile) ListDevJobs(releasePaths, releaseNames, releaseVersions []string, cacheDir string) error {

	releases, err := loadDevReleases(releasePaths, releaseNames, releaseVersions, cacheDir)
	if err != nil {
		return err
	}

	for _, release := range releases {
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
func (f *Fissile) CompileDev(releasePaths, releaseNames, releaseVersions []string, cacheDir, repository, targetPath string, workerCount int) error {
	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	releases, err := loadDevReleases(releasePaths, releaseNames, releaseVersions, cacheDir)
	if err != nil {
		return err
	}

	for _, release := range releases {
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
func (f *Fissile) GenerateRoleDevImages(targetPath, repository string, noBuild, force bool, releasePaths, releaseNames, releaseVersions []string, cacheDir, rolesManifestPath, compiledPackagesPath, defaultConsulAddress, defaultConfigStorePrefix string) error {
	releases, err := loadDevReleases(releasePaths, releaseNames, releaseVersions, cacheDir)
	if err != nil {
		return err
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, releases)
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
		defaultConsulAddress,
		defaultConfigStorePrefix,
		"",
		f.Version,
		f.ui,
	)

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
		dockerfileDir, err := roleBuilder.CreateDockerfileDir(role)
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
func (f *Fissile) ListDevRoleImages(repository string, releasePaths, releaseNames, releaseVersions []string, cacheDir, rolesManifestPath string, existingOnDocker, withVirtualSize bool) error {
	if withVirtualSize && !existingOnDocker {
		return fmt.Errorf("Cannot list image virtual sizes if not matching image names with docker")
	}

	releases, err := loadDevReleases(releasePaths, releaseNames, releaseVersions, cacheDir)
	if err != nil {
		return err
	}

	var dockerManager *docker.ImageManager

	if existingOnDocker {
		dockerManager, err = docker.NewImageManager()
		if err != nil {
			return fmt.Errorf("Error connecting to docker: %s", err.Error())
		}
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, releases)
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
func (f *Fissile) GenerateDevConfigurationBase(releasePaths, releaseNames, releaseVersions []string, cacheDir, rolesManifestPath, lightManifestPath, darkManifestPath, targetPath, prefix, provider string) error {

	releases, err := loadDevReleases(releasePaths, releaseNames, releaseVersions, cacheDir)
	if err != nil {
		return err
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	configStore := configstore.NewConfigStoreBuilder(prefix, provider, lightManifestPath, darkManifestPath, targetPath)

	if err := configStore.WriteBaseConfig(rolesManifest); err != nil {
		return fmt.Errorf("Error writing base config: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Done."))

	return nil
}

func loadDevReleases(releasePaths, releaseNames, releaseVersions []string, cacheDir string) ([]*model.Release, error) {
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
			return nil, fmt.Errorf("Error loading release information: %s", err.Error())
		}

		releases[idx] = release
	}

	return releases, nil
}

/*
// DiffDevConfigurationBases generates a diff comparing the opinions and supplied stubs for two different BOSH releases
func (f *Fissile) DiffDevConfigurationBases(releasePath1, lightManifestPath1, darkManifestPath1, releasePath2, lightManifestPath2, darkManifestPath2, targetPath, prefix, provider string) error {
	releases, err := loadDevReleases([]string{releasePath1, releasePath2}, releaseNames, releaseVersions, cacheDir)
	if err != nil {
		return fmt.Errorf("Error loading release information for release path %s: %s", releasePath1, err.Error())
	}

	configStore1 := configstore.NewConfigStoreBuilder(prefix, provider, lightManifestPath1, darkManifestPath1, targetPath)
	configStore2 := configstore.NewConfigStoreBuilder(prefix, provider, lightManifestPath2, darkManifestPath2, targetPath)

	if err := configStore.WriteBaseConfig(releases); err != nil {
		return fmt.Errorf("Error writing base config: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Done."))

	return nil
}
*/
