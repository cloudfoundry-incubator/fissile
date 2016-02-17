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
	"gopkg.in/yaml.v2"
)

// Fissile represents a fissile application
type Fissile struct {
	Version string
	ui      *termui.UI
	cmdErr  error
}

// NewFissileApplication creates a new app.Fissile
func NewFissileApplication(version string, ui *termui.UI) *Fissile {
	return &Fissile{
		Version: version,
		ui:      ui,
	}
}

// ListPackages will list all BOSH packages within a release
func (f *Fissile) ListPackages(releasePath string) error {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		return fmt.Errorf("Error loading release information: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, pkg := range release.Packages {
		f.ui.Printf("%s (%s)\n", color.YellowString(pkg.Name), color.WhiteString(pkg.Version))
	}

	f.ui.Printf(
		"There are %s packages present.\n",
		color.GreenString("%d", len(release.Packages)),
	)

	return nil
}

// ListJobs will list all jobs within a release
func (f *Fissile) ListJobs(releasePath string) error {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		return fmt.Errorf("Error loading release information: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, job := range release.Jobs {
		f.ui.Printf("%s (%s): %s\n", color.YellowString(job.Name), color.WhiteString(job.Version), job.Description)
	}

	f.ui.Printf(
		"There are %s jobs present.\n",
		color.GreenString("%d", len(release.Jobs)),
	)

	return nil
}

// VerifyRelease checks that the release files match the checksum in the release manifest
func (f *Fissile) VerifyRelease(releasePath string) error {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		return fmt.Errorf("Error loading release information: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	for _, job := range release.Jobs {
		if err := job.ValidateSHA1(); err != nil {
			return err
		}
		f.ui.Printf("Verified job %s\n", color.YellowString(job.Name))
	}

	for _, pkg := range release.Packages {
		if err := pkg.ValidateSHA1(); err != nil {
			return err
		}
		f.ui.Printf("Verified package %s\n", color.YellowString(pkg.Name))
	}

	if release.License.SHA1 != "" {
		if release.License.ActualSHA1 == "" {
			f.ui.Printf("WARNING: Skipping license integrity check, no %s.\n", color.YellowString("license.tgz"))
		} else {
			if release.License.SHA1 != release.License.ActualSHA1 {
				return fmt.Errorf("Computed SHA1 (%s) is different than manifest sha1 (%s) for license file\n", release.License.ActualSHA1, release.License.SHA1)
			}
			f.ui.Println("Verified license file")
		}
	}

	f.ui.Printf("Release %s (%s) verified successfully\n", color.GreenString(release.Name), color.YellowString(release.Path))

	return nil
}

// ListFullConfiguration will output all the configurations within the release
// TODO this should be updated to use release.GetUniqueConfigs
func (f *Fissile) ListFullConfiguration(releasePath string) error {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		return fmt.Errorf("Error loading release information: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	propertiesGroupedUsageCounts := map[string]int{}
	propertiesGroupedDefaults := map[string][]interface{}{}

	for _, job := range release.Jobs {
		for _, property := range job.Properties {

			if _, ok := propertiesGroupedUsageCounts[property.Name]; ok {
				propertiesGroupedUsageCounts[property.Name]++
			} else {
				propertiesGroupedUsageCounts[property.Name] = 1
				propertiesGroupedDefaults[property.Name] = []interface{}{}
			}

			if property.Default != nil {
				propertiesGroupedDefaults[property.Name] = append(propertiesGroupedDefaults[property.Name], property.Default)
			}
		}
	}

	keys := make([]string, 0, len(propertiesGroupedUsageCounts))
	for k := range propertiesGroupedUsageCounts {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	keysWithDefaults := 0

	for _, name := range keys {
		f.ui.Printf(
			"====== %s ======\nUsage count: %s\n",
			color.GreenString(name),
			color.MagentaString("%d", propertiesGroupedUsageCounts[name]),
		)

		defaults := propertiesGroupedDefaults[name]

		if len(defaults) > 0 {
			buf, err := yaml.Marshal(defaults[0])
			if err != nil {
				return fmt.Errorf("Error marshaling config value %v: %s", defaults[0], err.Error())
			}
			previous := string(buf)
			f.ui.Printf(
				"Default:\n%s\n",
				color.YellowString(previous),
			)

			for _, value := range defaults[1:] {
				buf, err := yaml.Marshal(value)
				if err != nil {
					return fmt.Errorf("Error marshaling config value %v: %s", value, err.Error())
				}
				current := string(buf)
				if current != previous {
					f.ui.Printf(
						"*** ALTERNATE DEFAULT:\n%s\n",
						color.RedString(current),
					)
				}
				previous = current
			}
		}

		if len(propertiesGroupedDefaults[name]) > 0 {
			keysWithDefaults++
		}
	}

	f.ui.Printf(
		"There are %s unique configuration keys present. %s of them have default values.\n",
		color.GreenString("%d", len(propertiesGroupedUsageCounts)),
		color.GreenString("%d", keysWithDefaults),
	)

	return nil
}

// PrintTemplateReport will output details about the contents of a release
func (f *Fissile) PrintTemplateReport(releasePath string) error {
	release, err := model.NewRelease(releasePath)
	if err != nil {
		return fmt.Errorf("Error loading release information: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

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
				f.ui.Println(color.RedString("Error reading template blocks for template %s in job %s: %s", template.SourcePath, job.Name, err.Error()))
			}

			for _, block := range blocks {
				switch {
				case block.Type == model.TextBlock:
					countText++
				case block.Type == model.PrintBlock:
					countPrint++

					transformedBlock, err := block.Transform()
					if err != nil {
						f.ui.Println(color.RedString("Error transforming block %s for template %s in job %s: %s", block.Block, template.SourcePath, job.Name, err.Error()))
					}

					if transformedBlock != "" {
						countPrintTransformed++
					}

				case block.Type == model.CodeBlock:
					countCode++

					transformedBlock, err := block.Transform()
					if err != nil {
						f.ui.Println(color.RedString("Error transforming block %s for template %s in job %s: %s", block.Block, template.SourcePath, job.Name, err.Error()))
					}

					if transformedBlock != "" {
						countCodeTransformed++
					}
				}
			}
		}
	}

	f.ui.Printf(
		"There are %s templates present.\n",
		color.GreenString("%d", templateCount),
	)

	f.ui.Printf(
		"There are %s text blocks that we don't need to touch.\n",
		color.GreenString("%d", countText),
	)

	f.ui.Printf(
		"There are %s print blocks, and we can transform %s of them.\n",
		color.MagentaString("%d", countPrint),
		color.GreenString("%d", countPrintTransformed),
	)

	f.ui.Printf(
		"There are %s code blocks, and we can transform %s of them.\n",
		color.MagentaString("%d", countCode),
		color.GreenString("%d", countCodeTransformed),
	)

	return nil
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

// Compile will compile a full BOSH release
func (f *Fissile) Compile(releasePath, repository, targetPath string, workerCount int, keepContainer bool) error {
	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	release, err := model.NewRelease(releasePath)
	if err != nil {
		return fmt.Errorf("Error loading release information: %s", err.Error())
	}

	f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))

	comp, err := compilator.NewCompilator(dockerManager, targetPath, repository, compilation.UbuntuBase, f.Version, keepContainer, f.ui)
	if err != nil {
		return fmt.Errorf("Error creating a new compilator: %s", err.Error())
	}

	if err := comp.Compile(workerCount, release); err != nil {
		return fmt.Errorf("Error compiling packages: %s", err.Error())
	}

	return nil
}

//GenerateConfigurationBase generates a configuration base using a BOSH release and opinions from manifests
func (f *Fissile) GenerateConfigurationBase(releasePaths []string, lightManifestPath, darkManifestPath, rolesManifestPath, targetPath, prefix, provider string) error {
	releases := make([]*model.Release, len(releasePaths))
	for idx, releasePath := range releasePaths {
		release, err := model.NewRelease(releasePath)
		if err != nil {
			return fmt.Errorf("Error loading release information: %s", err.Error())
		}
		releases[idx] = release
		f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))
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

// GenerateRoleImages generates all role images
func (f *Fissile) GenerateRoleImages(targetPath, repository string, noBuild bool, releasePaths []string, rolesManifestPath, compiledPackagesPath, defaultConsulAddress, defaultConfigStorePrefix, version string, workerCount int) error {
	releases := make([]*model.Release, len(releasePaths))
	for idx, releasePath := range releasePaths {
		release, err := model.NewRelease(releasePath)
		if err != nil {
			return fmt.Errorf("Error loading release information: %s", err.Error())
		}
		releases[idx] = release
		f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))
	}

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	roleBuilder := builder.NewRoleImageBuilder(
		repository,
		compiledPackagesPath,
		targetPath,
		defaultConsulAddress,
		defaultConfigStorePrefix,
		version,
		f.Version,
		f.ui,
	)

	if err = roleBuilder.BuildRoleImages(rolesManifest.Roles, repository, version, noBuild, workerCount); err != nil {
		return err
	}

	f.ui.Println(color.GreenString("Done."))

	return err
}

// ListRoleImages lists all role images
func (f *Fissile) ListRoleImages(repository string, releasePaths []string, rolesManifestPath, version string, existingOnDocker, withVirtualSize bool) error {
	if withVirtualSize && !existingOnDocker {
		return fmt.Errorf("Cannot list image virtual sizes if not matching image names with docker")
	}

	releases := make([]*model.Release, len(releasePaths))
	for idx, releasePath := range releasePaths {
		release, err := model.NewRelease(releasePath)
		if err != nil {
			return fmt.Errorf("Error loading release information: %s", err.Error())
		}
		releases[idx] = release
		f.ui.Println(color.GreenString("Release %s loaded successfully", color.YellowString(release.Name)))
	}

	var dockerManager *docker.ImageManager
	var err error

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
		imageName := builder.GetRoleImageName(repository, role, version)

		if existingOnDocker {
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
		} else {
			f.ui.Println(imageName)
		}
	}

	return nil
}

type keyHash map[string]string

// HashDiffs summarizes the diffs between the two configs
type HashDiffs struct {
	AddedKeys     []string
	DeletedKeys   []string
	ChangedValues map[string][2]string
}

// DiffConfigurationBases generates a diff comparing the opinions and supplied stubs for two different BOSH releases
func (f *Fissile) DiffConfigurationBases(releasePaths []string, prefix string) error {
	hashDiffs, err := f.GetDiffConfigurationBases(releasePaths, prefix)
	if err != nil {
		return err
	}
	f.reportHashDiffs(hashDiffs)
	return nil
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

// GetDiffConfigurationBases calcs the difference in configs and returns a hash
func (f *Fissile) GetDiffConfigurationBases(releasePaths []string, prefix string) (*HashDiffs, error) {
	var err error
	if len(releasePaths) != 2 {
		return nil, fmt.Errorf("configuration diff: expected two release paths, got %d", len(releasePaths))
	}
	releases := make([]*model.Release, 2)
	for idx, releasePath := range releasePaths {
		releases[idx], err = model.NewRelease(releasePath)
		if err != nil {
			return nil, fmt.Errorf("Error loading release information for path %s: %s", releasePath, err.Error())
		}
	}
	return getDiffsFromReleases(releases, prefix)
}

func getDiffsFromReleases(releases []*model.Release, prefix string) (*HashDiffs, error) {
	hashes := [2]keyHash{keyHash{}, keyHash{}}
	for idx, release := range releases {
		configs := release.GetUniqueConfigs()
		for _, config := range configs {
			key, err := configstore.BoshKeyToConsulPath(config.Name, configstore.DescriptionsStore, prefix)
			if err != nil {
				return nil, fmt.Errorf("Error getting config %s for release %s: %s", config.Name, release.Name, err.Error())
			}
			hashes[idx][key] = config.Description
		}
		// Get the spec configs
		for _, job := range release.Jobs {
			for _, property := range job.Properties {
				key, err := configstore.BoshKeyToConsulPath(fmt.Sprintf("%s.%s.%s", release.Name, job.Name, property.Name), configstore.SpecStore, prefix)
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
