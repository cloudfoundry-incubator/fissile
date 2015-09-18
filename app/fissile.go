package app

import (
	"fmt"
	"log"

	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"

	"github.com/fatih/color"
)

type Fissile interface {
	ListPackages(releasePath string)
	ListJobs(releasePath string)
	ListFullConfiguration(releasePath string)
	ShowBaseImage(dockerEndpoint string, baseImage string)
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

func (f *FissileApp) ShowBaseImage(dockerEndpoint string, baseImage string) {
	dockerManager, err := docker.NewDockerImageManager(dockerEndpoint)
	if err != nil {
		log.Fatalln(color.RedString("Error connecting to docker: %s", err.Error()))
	}

	image, err := dockerManager.FindBaseImage(baseImage)
	if err != nil {
		log.Fatalln(color.RedString("Error looking up base image %s: %s", baseImage, err.Error()))
	}

	log.Printf("ID: %s", color.GreenString(image.ID))
	log.Printf("Virtual Size: %sMB", color.YellowString(fmt.Sprintf("%.2f", float64(image.VirtualSize)/(1024*1024))))
}
