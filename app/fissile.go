package app

import (
	"bytes"
	"fmt"
	"os"
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
