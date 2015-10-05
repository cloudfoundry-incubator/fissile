package app

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/hpcloud/fissile/compilator"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"

	"github.com/fatih/color"
)

type Fissile interface {
	ListPackages(releasePath string)
	ListJobs(releasePath string)
	ListFullConfiguration(releasePath string)
	PrintAllTemplateContents(releasePath string)
	ShowBaseImage(dockerEndpoint string, baseImage string)
	CreateBaseCompilationImage(dockerEndpoint, baseImage, releasePath, prefix string)
	Compile(dockerEndpoint, baseImageName, releasePath, repository, targetPath string, workerCount int)
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

func (f *FissileApp) PrintAllTemplateContents(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))
	count := 0
	countTransformed := 0
	for _, job := range release.Jobs {
		for _, template := range job.Templates {
			blocks, err := template.GetErbBlocks()

			if err != nil {
				log.Println(color.RedString("Error reading template blocks for template %s in job %s: %s", template.SourcePath, job.Name, err.Error()))
			}

			for _, block := range blocks {
				if block.Type == "print" {
					count++

					transformedBlock, err := block.Transform()
					if err != nil {
						log.Println(color.RedString("Error transforming block %s for template %s in job %s: %s", block.Block, template.SourcePath, job.Name, err.Error()))
					}

					if transformedBlock != "" {
						countTransformed++
					} else {
						log.Println(color.MagentaString(block.Block))
					}
				}
			}

		}
	}
	log.Printf("Transformed %d out of %d", countTransformed, count)
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

	tempDir, err := ioutil.TempDir("", "fissile-base-compiler")
	if err != nil {
		log.Fatalln(color.RedString("Error creating temp dir: %s", err.Error()))
	}

	comp, err := compilator.NewCompilator(dockerManager, release, tempDir, repository, compilation.UbuntuBase)
	if err != nil {
		log.Fatalln(color.RedString("Error creating a new compilator: %s", err.Error()))
	}

	if _, err := comp.CreateCompilationBase(baseImageName); err != nil {
		log.Fatalln(color.RedString("Error creating compilation base image: %s", err.Error()))
	}
}

func (f *FissileApp) Compile(dockerEndpoint, baseImageName, releasePath, repository, targetPath string, workerCount int) {
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

	comp, err := compilator.NewCompilator(dockerManager, release, targetPath, repository, compilation.UbuntuBase)
	if err != nil {
		log.Fatalln(color.RedString("Error creating a new compilator: %s", err.Error()))
	}

	if _, err := comp.CreateCompilationBase(baseImageName); err != nil {
		log.Fatalln(color.RedString("Error creating compilation base image: %s", err.Error()))
	}

	if err := comp.Compile(workerCount); err != nil {
		log.Fatalln(color.RedString("Error compiling packages: %s", err.Error()))
	}
}
