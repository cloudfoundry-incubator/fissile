package app

import (
	"fmt"
	"log"

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
