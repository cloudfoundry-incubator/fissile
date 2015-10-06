package app

import (
	"fmt"
	"io/ioutil"
	"log"
	"sort"

	"github.com/hpcloud/fissile/compilator"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"

	"github.com/fatih/color"
)

func ListPackages(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, pkg := range release.Packages {
		log.Printf("%s (%s)\n", color.YellowString(pkg.Name), color.WhiteString(pkg.Version))
	}

	log.Printf(
		"There are %s packages present.",
		color.GreenString(fmt.Sprintf("%d", len(release.Packages))),
	)
}

func ListJobs(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, job := range release.Jobs {
		log.Printf("%s (%s): %s\n", color.YellowString(job.Name), color.WhiteString(job.Version), job.Description)
	}

	log.Printf(
		"There are %s jobs present.",
		color.GreenString(fmt.Sprintf("%d", len(release.Jobs))),
	)
}

func ListFullConfiguration(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	propertiesGrouped := map[string]int{}

	for _, job := range release.Jobs {
		for _, property := range job.Properties {

			if _, ok := propertiesGrouped[property.Name]; ok {
				propertiesGrouped[property.Name]++
			} else {
				propertiesGrouped[property.Name] = 1
			}
		}
	}

	keys := make([]string, 0, len(propertiesGrouped))
	for k := range propertiesGrouped {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, name := range keys {
		log.Printf(
			"Count: %s\t%s",
			color.MagentaString(fmt.Sprintf("%d", propertiesGrouped[name])),
			color.YellowString(name),
		)
	}

	log.Printf(
		"There are %s configuration keys present.",
		color.GreenString(fmt.Sprintf("%d", len(propertiesGrouped))),
	)
}

func PrintTemplateReport(releasePath string) {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		log.Fatalln(color.RedString("Error loading release information: %s", err.Error()))
	}

	log.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	templateCount := 0

	countPrint := 0
	countPrintTransformed := 0

	countText := 0

	countCode := 0
	countCodeTransformed := 0

	for _, job := range release.Jobs {
		for _, template := range job.Templates {
			templateCount++

			blocks, err := template.GetErbBlocks()

			if err != nil {
				log.Println(color.RedString("Error reading template blocks for template %s in job %s: %s", template.SourcePath, job.Name, err.Error()))
			}

			for _, block := range blocks {
				switch {
				case block.Type == model.TextBlock:
					countText++
				case block.Type == model.PrintBlock:
					countPrint++

					transformedBlock, err := block.Transform()
					if err != nil {
						log.Println(color.RedString("Error transforming block %s for template %s in job %s: %s", block.Block, template.SourcePath, job.Name, err.Error()))
					}

					if transformedBlock != "" {
						countPrintTransformed++
					}

				case block.Type == model.CodeBlock:
					countCode++

					transformedBlock, err := block.Transform()
					if err != nil {
						log.Println(color.RedString("Error transforming block %s for template %s in job %s: %s", block.Block, template.SourcePath, job.Name, err.Error()))
					}

					if transformedBlock != "" {
						countCodeTransformed++
					}
				}
			}

		}
	}

	log.Printf(
		"There are %s templates present.",
		color.GreenString("%d", templateCount),
	)

	log.Printf(
		"There are %s text blocks that we don't need to touch.",
		color.GreenString("%d", countText),
	)

	log.Printf(
		"There are %s print blocks, and we can transform %s of them.",
		color.MagentaString("%d", countPrint),
		color.GreenString("%d", countPrintTransformed),
	)

	log.Printf(
		"There are %s code blocks, and we can transform %s of them.",
		color.MagentaString("%d", countCode),
		color.GreenString("%d", countCodeTransformed),
	)
}

func ShowBaseImage(dockerEndpoint, baseImage string) {
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

func CreateBaseCompilationImage(dockerEndpoint, baseImageName, releasePath, repository string) {
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

func Compile(dockerEndpoint, baseImageName, releasePath, repository, targetPath string, workerCount int) {
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
