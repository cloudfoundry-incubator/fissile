package app

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hpcloud/fissile/builder"
	"github.com/hpcloud/fissile/compilator"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"

	"github.com/fatih/color"
)

// ListDevPackages will list all BOSH packages within a list of dev releases
func (f *Fissile) ListDevPackages(releasePaths, releaseNames, releaseVersions []string, cacheDir string) {

	for idx, releasePath := range releasePaths {
		var releaseName, releaseVersion string

		if releaseName = ""; len(releaseNames) != 0 {
			releaseName = releaseNames[idx]
		}

		if releaseVersion = ""; len(releaseVersions) != 0 {
			releaseVersion = releaseVersions[idx]
		}

		release, err := model.NewDevRelease(releasePath, releaseName, releaseVersion, cacheDir)
		if err != nil {
			log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
		}

		log.Println(color.GreenString("Dev release %s (%s) loaded successfully", color.YellowString(release.Name), color.MagentaString(release.Version)))

		for _, pkg := range release.Packages {
			log.Printf("%s (%s)\n", color.YellowString(pkg.Name), color.WhiteString(pkg.Version))
		}

		log.Printf(
			"There are %s packages present.\n\n",
			color.GreenString(fmt.Sprintf("%d", len(release.Packages))),
		)
	}
}

// ListDevJobs will list all jobs within a list of dev releases
func (f *Fissile) ListDevJobs(releasePaths, releaseNames, releaseVersions []string, cacheDir string) {
	for idx, releasePath := range releasePaths {
		var releaseName, releaseVersion string

		if releaseName = ""; len(releaseNames) != 0 {
			releaseName = releaseNames[idx]
		}

		if releaseVersion = ""; len(releaseVersions) != 0 {
			releaseVersion = releaseVersions[idx]
		}

		release, err := model.NewDevRelease(releasePath, releaseName, releaseVersion, cacheDir)
		if err != nil {
			log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
		}

		log.Println(color.GreenString("Dev release %s (%s) loaded successfully", color.YellowString(release.Name), color.MagentaString(release.Version)))

		for _, job := range release.Jobs {
			log.Printf("%s (%s): %s\n", color.YellowString(job.Name), color.WhiteString(job.Version), job.Description)
		}

		log.Printf(
			"There are %s jobs present.\n\n",
			color.GreenString(fmt.Sprintf("%d", len(release.Jobs))),
		)
	}
}

// CompileDev will compile a list of dev BOSH releases
func (f *Fissile) CompileDev(releasePaths, releaseNames, releaseVersions []string, cacheDir, repository, targetPath string, workerCount int) {
	dockerManager, err := docker.NewImageManager()
	if err != nil {
		log.Fatalln(color.RedString("Error connecting to docker: %s", err.Error()))
	}

	for idx, releasePath := range releasePaths {
		var releaseName, releaseVersion string

		if releaseName = ""; len(releaseNames) != 0 {
			releaseName = releaseNames[idx]
		}

		if releaseVersion = ""; len(releaseVersions) != 0 {
			releaseVersion = releaseVersions[idx]
		}

		release, err := model.NewDevRelease(releasePath, releaseName, releaseVersion, cacheDir)
		if err != nil {
			log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
		}

		log.Println(color.GreenString("Compiling packages for release %s (%s) ...", color.YellowString(release.Name), color.MagentaString(release.Version)))

		comp, err := compilator.NewCompilator(dockerManager, targetPath, repository, compilation.UbuntuBase, f.Version)
		if err != nil {
			log.Fatalln(color.RedString("Error creating a new compilator: %s", err.Error()))
		}

		if err := comp.Compile(workerCount, release); err != nil {
			log.Fatalln(color.RedString("Error compiling packages: %s", err.Error()))
		}
	}
}

// GenerateRoleDevImages generates all role images using dev releases
func (f *Fissile) GenerateRoleDevImages(targetPath, repository string, noBuild bool, releasePaths, releaseNames, releaseVersions []string, cacheDir, rolesManifestPath, compiledPackagesPath, defaultConsulAddress, defaultConfigStorePrefix string) {
	releases := make([]*model.Release, len(releasePaths))
	for idx, releasePath := range releasePaths {
		var releaseName, releaseVersion string

		if releaseName = ""; len(releaseNames) != 0 {
			releaseName = releaseNames[idx]
		}

		if releaseVersion = ""; len(releaseVersions) != 0 {
			releaseVersion = releaseVersions[idx]
		}

		release, err := model.NewDevRelease(releasePath, releaseName, releaseVersion, cacheDir)
		if err != nil {
			log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
		}

		log.Println(color.GreenString("Dev release %s (%s) loaded successfully", color.YellowString(release.Name), color.MagentaString(release.Version)))

		releases[idx] = release
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, releases)
	if err != nil {
		log.Fatalln(color.RedString("Error loading roles manifest: %s", err.Error()))
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		log.Fatalln(color.RedString("Error connecting to docker: %s", err.Error()))
	}

	roleBuilder := builder.NewRoleImageBuilder(
		repository,
		compiledPackagesPath,
		targetPath,
		defaultConsulAddress,
		defaultConfigStorePrefix,
		"",
		f.Version,
	)

	for _, role := range rolesManifest.Roles {
		roleImageName, err := builder.GetRoleDevImageName(repository, role, role.GetRoleDevVersion())
		if err != nil {
			log.Fatalln(color.RedString("Error generating image name for role %s: %s", role.Name, err.Error()))
		}

		_, err = dockerManager.FindImage(roleImageName)
		if err == nil {
			log.Printf("Dev image %s for role %s already exists. Skipping ...\n", roleImageName, color.YellowString(role.Name))
			continue
		}

		// Remove existing Dockerfile directory
		roleDir := filepath.Join(targetPath, role.Name)
		if err := os.RemoveAll(roleDir); err != nil {
			log.Fatalln(color.RedString("Error removing Dockerfile directory and/or assets for role %s: %s", role.Name, err.Error()))
		}

		log.Printf("Creating Dockerfile for role %s ...\n", color.YellowString(role.Name))
		dockerfileDir, err := roleBuilder.CreateDockerfileDir(role)
		if err != nil {
			log.Fatalln(color.RedString("Error creating Dockerfile and/or assets for role %s: %s", role.Name, err.Error()))
		}

		if !noBuild {
			if !strings.HasSuffix(dockerfileDir, string(os.PathSeparator)) {
				dockerfileDir = fmt.Sprintf("%s%c", dockerfileDir, os.PathSeparator)
			}

			log.Printf("Building docker image in %s ...\n", color.YellowString(dockerfileDir))

			if err := dockerManager.BuildImage(
				dockerfileDir,
				roleImageName,
				func(stdout io.Reader) {
					scanner := bufio.NewScanner(stdout)
					for scanner.Scan() {
						log.Println(color.GreenString("build-%s > %s", color.MagentaString(roleImageName), color.WhiteString(scanner.Text())))
					}
				},
			); err != nil {
				log.Fatalln(color.RedString("Error building base image: %s", err.Error()))
			}
		} else {
			log.Println("Skipping image build because of flag.")
		}
	}

	log.Println(color.GreenString("Done."))
}
