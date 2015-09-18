package app

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/hpcloud/fissile/baseos/compilation"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"

	"github.com/fatih/color"
)

type Fissile interface {
	ListPackages(releasePath string)
	ListJobs(releasePath string)
	ListFullConfiguration(releasePath string)
	ShowBaseImage(dockerEndpoint string, baseImage string)
	CreateBaseCompilationImage(dockerEndpoint, baseImage, releasePath, prefix string)
}

type FissileApp struct {
}

func NewFissileApp() Fissile {
	return &FissileApp{}
}

func (f *FissileApp) ListPackages(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, pkg := range release.Packages {
		log.Printf("%s (%s)\n", color.YellowString(pkg.Name), color.WhiteString(pkg.Version))
	}
}

func (f *FissileApp) ListJobs(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, job := range release.Jobs {
		log.Printf("%s (%s): %s\n", color.YellowString(job.Name), color.WhiteString(job.Version), job.Description)
	}
}

func (f *FissileApp) ListFullConfiguration(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, job := range release.Jobs {
		for _, property := range job.Properties {
			log.Printf(
				"%s (%s): %s\n",
				color.YellowString(property.Name),
				color.WhiteString("%v", property.Default),
				property.Description,
			)
		}
	}
}

func (f *FissileApp) ShowBaseImage(dockerEndpoint, baseImage string) {
	dockerManager, err := docker.NewDockerImageManager(dockerEndpoint)
	if err != nil {
		log.Fatalln(color.RedString("Error connecting to docker: %s", err.Error()))
	}

	image, err := dockerManager.FindImage(baseImage)
	if err != nil {
		log.Fatalln(color.RedString("Error looking up base image %s: %s", baseImage, err.Error()))
	}

	log.Printf("ID: %s", color.GreenString(image.ID))
	log.Printf("Virtual Size: %sMB", color.YellowString(fmt.Sprintf("%.2f", float64(image.VirtualSize)/(1024*1024))))
}

func (f *FissileApp) CreateBaseCompilationImage(dockerEndpoint, baseImageName, releasePath, repository string) {
	dockerManager, err := docker.NewDockerImageManager(dockerEndpoint)
	if err != nil {
		log.Fatalln(color.RedString("Error connecting to docker: %s", err.Error()))
	}

	baseImage, err := dockerManager.FindImage(baseImageName)
	if err != nil {
		log.Fatalln(color.RedString("Error looking up base image %s: %s", baseImage, err.Error()))
	}

	log.Println(color.GreenString("Base image with ID %s found", color.YellowString(baseImage.ID)))

	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	imageName := getBaseCompilationImageName(repository, release.Name, release.Version)
	log.Println(color.GreenString("Using %s as a compilation image name", color.YellowString(imageName)))

	containerName := getBaseCompilationContainerName(repository, release.Name, release.Version)
	log.Println(color.GreenString("Using %s as a compilation container name", color.YellowString(containerName)))

	image, err := dockerManager.FindImage(imageName)
	if err != nil {
		log.Println("Image doesn't exist, it will be created ...")
	} else {
		log.Println(color.GreenString(
			"Compilation image %s with ID %s already exists. Doing nothing.",
			color.YellowString(imageName),
			color.YellowString(image.ID),
		))
	}

	tempScriptDir, err := ioutil.TempDir("", "fissile-compilation")
	if err != nil {
		log.Fatalln(color.RedString("Could not create temp dir %s: %s", tempScriptDir, err.Error()))
	}

	compilationScript, err := compilation.Asset("baseos/compilation/ubuntu.sh")
	if err != nil {
		log.Fatalln(color.RedString("Error loading script asset. This is probably a bug: %s", err.Error()))
	}

	targetScriptName := "compilation-prerequisites.sh"
	containerScriptPath := filepath.Join(docker.ContainerInPath, targetScriptName)

	hostScriptPath := filepath.Join(tempScriptDir, "compilation-prerequisites.sh")
	if err = ioutil.WriteFile(hostScriptPath, compilationScript, 0700); err != nil {
		log.Fatalln(color.RedString("Error saving script asset: %s", err.Error()))
	}

	exitCode, container, err := dockerManager.RunInContainer(
		containerName,
		baseImageName,
		[]string{"bash", "-c", containerScriptPath},
		tempScriptDir,
		"",
		func(stdout io.Reader) {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				log.Println(color.GreenString("compilation-container > %s", color.WhiteString(scanner.Text())))
			}
		},
		func(stderr io.Reader) {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Println(color.GreenString("compilation-container > %s", color.RedString(scanner.Text())))
			}
		},
	)
	defer func() {
		if container != nil {
			err = dockerManager.RemoveContainer(container.ID)
			if err != nil {
				log.Fatalln(color.RedString("Error removing container %s: %s", container.ID, err.Error()))
			}
		}
	}()

	if err != nil {
		log.Fatalln(color.RedString("Error running script: %s", err.Error()))
	}

	if exitCode != 0 {
		log.Fatalln(color.RedString("Error - script script exited with code %d: %s", exitCode, err.Error()))
	}

}

func getBaseCompilationContainerName(repository, releaseName, releaseVersion string) string {
	return fmt.Sprintf("%s-%s-%s-cbase", repository, releaseName, releaseVersion)
}

func getBaseCompilationImageName(repository, releaseName, releaseVersion string) string {
	return fmt.Sprintf("%s:%s-%s-cbase", repository, releaseName, releaseVersion)
}
