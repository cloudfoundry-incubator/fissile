package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hpcloud/fissile/builder"
	"github.com/hpcloud/fissile/compilator"
	"github.com/hpcloud/fissile/docker"
	"github.com/hpcloud/fissile/kube"
	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/scripts/compilation"
	"github.com/hpcloud/fissile/util"
	"github.com/hpcloud/fissile/validation"

	"github.com/fatih/color"
	"github.com/hpcloud/stampy"
	"github.com/hpcloud/termui"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

// Fissile represents a fissile application
type Fissile struct {
	Version                    string
	UI                         *termui.UI
	cmdErr                     error
	releases                   []*model.Release // Only applies for some commands
	patchPropertiesReleaseName string           // Only applies for some commands
	patchPropertiesJobName     string           // Only applies for some commands
}

// NewFissileApplication creates a new app.Fissile
func NewFissileApplication(version string, ui *termui.UI) *Fissile {
	return &Fissile{
		Version: version,
		UI:      ui,
	}
}

// SetPatchPropertiesDirective saves the patch-properties release and job names, if specified.
func (f *Fissile) SetPatchPropertiesDirective(patchPropertiesDirective string) error {
	if patchPropertiesDirective == "" {
		return nil
	}
	msgStart := "Invalid format for --patch-properties-release flag: should be RELEASE/JOB;"
	parts := strings.Split(patchPropertiesDirective, "/")
	if len(parts) != 2 {
		return fmt.Errorf(msgStart+" got %d part(s)", len(parts))
	}
	if parts[0] == "" {
		return fmt.Errorf(msgStart + " no RELEASE is specified")
	}
	if parts[1] == "" {
		return fmt.Errorf(msgStart + " no JOB is specified")
	}
	f.patchPropertiesReleaseName = parts[0]
	f.patchPropertiesJobName = parts[1]
	return nil
}

// ShowBaseImage will show details about the base BOSH images
func (f *Fissile) ShowBaseImage(repository string) error {
	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	comp, err := compilator.NewCompilator(dockerManager, "", "", repository, compilation.UbuntuBase, f.Version, false, f.UI)
	if err != nil {
		return fmt.Errorf("Error creating a new compilator: %s", err.Error())
	}

	image, err := dockerManager.FindImage(comp.BaseImageName())
	if err != nil {
		return fmt.Errorf("Error looking up base image %s: %s", comp.BaseImageName(), err.Error())
	}

	f.UI.Printf("\nCompilation Layer: %s\n", color.GreenString(comp.BaseImageName()))
	f.UI.Printf("ID: %s\n", color.GreenString(image.ID))
	f.UI.Printf("Virtual Size: %sMB\n", color.YellowString("%.2f", float64(image.VirtualSize)/(1024*1024)))

	baseImageName := builder.GetBaseImageName(repository, f.Version)
	image, err = dockerManager.FindImage(baseImageName)
	f.UI.Printf("\nStemcell Layer: %s\n", color.GreenString(baseImageName))
	f.UI.Printf("ID: %s\n", color.GreenString(image.ID))
	f.UI.Printf("Virtual Size: %sMB\n", color.YellowString("%.2f", float64(image.VirtualSize)/(1024*1024)))

	return nil
}

// CreateBaseCompilationImage will recompile the base BOSH image for a release
func (f *Fissile) CreateBaseCompilationImage(baseImageName, repository, metricsPath string, keepContainer bool) error {
	if metricsPath != "" {
		stampy.Stamp(metricsPath, "fissile", "create-compilation-image", "start")
		defer stampy.Stamp(metricsPath, "fissile", "create-compilation-image", "done")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	baseImage, err := dockerManager.FindImage(baseImageName)
	if err != nil {
		return fmt.Errorf("Error looking up base image %s: %s", baseImageName, err)
	}

	f.UI.Println(color.GreenString("Base image with ID %s found", color.YellowString(baseImage.ID)))

	comp, err := compilator.NewCompilator(dockerManager, "", "", repository, compilation.UbuntuBase, f.Version, keepContainer, f.UI)
	if err != nil {
		return fmt.Errorf("Error creating a new compilator: %s", err.Error())
	}

	if _, err := comp.CreateCompilationBase(baseImageName); err != nil {
		return fmt.Errorf("Error creating compilation base image: %s", err.Error())
	}

	return nil
}

// GenerateBaseDockerImage generates a base docker image to be used as a FROM for role images
func (f *Fissile) GenerateBaseDockerImage(targetPath, baseImage, metricsPath string, noBuild bool, repository string) error {
	if metricsPath != "" {
		stampy.Stamp(metricsPath, "fissile", "create-role-base", "start")
		defer stampy.Stamp(metricsPath, "fissile", "create-role-base", "done")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	baseImageName := builder.GetBaseImageName(repository, f.Version)

	image, err := dockerManager.FindImage(baseImageName)
	if err == docker.ErrImageNotFound {
		f.UI.Println("Image doesn't exist, it will be created ...")
	} else if err != nil {
		return fmt.Errorf("Error looking up image: %s", err.Error())
	} else {
		f.UI.Println(color.GreenString(
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

	if noBuild {
		f.UI.Println("Skipping image build because of flag.")
		return nil
	}

	f.UI.Println("Building base docker image ...")
	log := new(bytes.Buffer)
	stdoutWriter := docker.NewFormattingWriter(
		log,
		docker.ColoredBuildStringFunc(baseImageName),
	)

	tarPopulator := baseImageBuilder.NewDockerPopulator()
	err = dockerManager.BuildImageFromCallback(baseImageName, stdoutWriter, tarPopulator)
	if err != nil {
		log.WriteTo(f.UI)
		return fmt.Errorf("Error building base image: %s", err)
	}
	f.UI.Println(color.GreenString("Done."))

	return nil
}

// ListPackages will list all BOSH packages within a list of dev releases
func (f *Fissile) ListPackages() error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.releases {
		f.UI.Println(color.GreenString("Dev release %s (%s)", color.YellowString(release.Name), color.MagentaString(release.Version)))

		for _, pkg := range release.Packages {
			f.UI.Printf("%s (%s)\n", color.YellowString(pkg.Name), color.WhiteString(pkg.Version))
		}

		f.UI.Printf(
			"There are %s packages present.\n\n",
			color.GreenString("%d", len(release.Packages)),
		)
	}

	return nil
}

// ListJobs will list all jobs within a list of dev releases
func (f *Fissile) ListJobs() error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.releases {
		f.UI.Println(color.GreenString("Dev release %s (%s)", color.YellowString(release.Name), color.MagentaString(release.Version)))

		for _, job := range release.Jobs {
			f.UI.Printf("%s (%s): %s\n", color.YellowString(job.Name), color.WhiteString(job.Version), job.Description)
		}

		f.UI.Printf(
			"There are %s jobs present.\n\n",
			color.GreenString("%d", len(release.Jobs)),
		)
	}

	return nil
}

// ListProperties will list all properties in all jobs within a list of dev releases
func (f *Fissile) ListProperties(outputFormat string) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	switch outputFormat {
	case "human":
		f.listPropertiesForHuman()
	case "json":
		// Note: The encoding/json is unable to handle type
		// -- map[interface {}]interface {}
		// Such types can occur when the default value has sub-structure.

		buf, err := util.JSONMarshal(f.collectProperties())
		if err != nil {
			return err
		}

		f.UI.Printf("%s", buf)
	case "yaml":
		buf, err := yaml.Marshal(f.collectProperties())
		if err != nil {
			return err
		}

		f.UI.Printf("%s", buf)
	default:
		return fmt.Errorf("Invalid output format '%s', expected one of human, json, or yaml", outputFormat)
	}

	return nil
}

func (f *Fissile) listPropertiesForHuman() {
	// Human readable output.
	for _, release := range f.releases {
		f.UI.Println(color.GreenString("Dev release %s (%s)",
			color.YellowString(release.Name), color.MagentaString(release.Version)))

		for _, job := range release.Jobs {
			f.UI.Printf("%s (%s): %s\n", color.YellowString(job.Name),
				color.WhiteString(job.Version), job.Description)

			for _, property := range job.Properties {
				f.UI.Printf("\t%s: %v\n", color.YellowString(property.Name),
					property.Default)
			}
		}
	}
}

func (f *Fissile) collectProperties() map[string]map[string]map[string]interface{} {
	// Generate a triple map (release -> job -> property -> default value)
	// which is easy to convert and dump to JSON or YAML.

	result := make(map[string]map[string]map[string]interface{})

	for _, release := range f.releases {
		result[release.Name] = make(map[string]map[string]interface{})

		for _, job := range release.Jobs {
			result[release.Name][job.Name] = make(map[string]interface{})
			for _, property := range job.Properties {
				result[release.Name][job.Name][property.Name] = property.Default
			}
		}
	}

	return result
}

// propertyDefaults is a double map
//
//	(property.name -> (default.string -> [*job...])
//
// which maps a property (name) to all its default values
// (stringified) and them in turn to the array of jobs where this
// default occurs.
type propertyDefaults map[string]map[string][]*model.Job

func (f *Fissile) collectPropertyDefaults() propertyDefaults {
	result := make(propertyDefaults)

	for _, release := range f.releases {
		for _, job := range release.Jobs {
			for _, property := range job.Properties {
				defaultAsString := fmt.Sprintf("%v", property.Default)

				if _, ok := result[property.Name]; !ok {
					result[property.Name] = make(map[string][]*model.Job)
				}
				if _, ok := result[property.Name][defaultAsString]; !ok {
					result[property.Name][defaultAsString] = make([]*model.Job, 1)
				}
				result[property.Name][defaultAsString] =
					append(result[property.Name][defaultAsString], job)
			}
		}
	}

	return result
}

// Compile will compile a list of dev BOSH releases
func (f *Fissile) Compile(repository, targetPath, roleManifestPath, metricsPath string, roleNames []string, workerCount int) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	if metricsPath != "" {
		stampy.Stamp(metricsPath, "fissile", "compile-packages", "start")
		defer stampy.Stamp(metricsPath, "fissile", "compile-packages", "done")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	f.UI.Println(color.GreenString("Compiling packages for dev releases:"))
	for _, release := range f.releases {
		f.UI.Printf("         %s (%s)\n", color.YellowString(release.Name), color.MagentaString(release.Version))
	}

	comp, err := compilator.NewCompilator(dockerManager, targetPath, metricsPath, repository, compilation.UbuntuBase, f.Version, false, f.UI)
	if err != nil {
		return fmt.Errorf("Error creating a new compilator: %s", err.Error())
	}

	roles, err := roleManifest.SelectRoles(roleNames)
	if err != nil {
		return fmt.Errorf("Error selecting packages to build: %s", err.Error())
	}

	if err := comp.Compile(workerCount, f.releases, roles); err != nil {
		return fmt.Errorf("Error compiling packages: %s", err.Error())
	}

	return nil
}

// CleanCache inspects the compilation cache and removes all packages
// which are not referenced (anymore).
func (f *Fissile) CleanCache(targetPath string) error {
	// 1. Collect list of packages referenced by the releases. A
	//    variant of the code in ListPackages, we keep only the
	//    hashes.

	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	referenced := make(map[string]int)
	for _, release := range f.releases {
		for _, pkg := range release.Packages {
			referenced[pkg.Version] = 1
		}
	}

	/// 2. Scan local compilation cache, compare to referenced,
	///    remove anything not found.

	f.UI.Printf("Cleaning up %s\n", color.MagentaString(targetPath))

	cached, err := filepath.Glob(targetPath + "/*")
	if err != nil {
		return err
	}

	removed := 0
	for _, cache := range cached {
		key := filepath.Base(cache)
		if _, ok := referenced[key]; ok {
			continue
		}

		f.UI.Printf("- Removing %s\n", color.YellowString(key))
		if err := os.RemoveAll(cache); err != nil {
			return err
		}
		removed++
	}

	if removed == 0 {
		f.UI.Println("Nothing found to remove")
		return nil
	}

	plural := ""
	if removed > 1 {
		plural = "s"
	}
	f.UI.Printf("Removed %s package%s\n",
		color.MagentaString(fmt.Sprintf("%d", removed)),
		plural)

	return nil
}

// GeneratePackagesRoleImage builds the docker image for the packages layer
// where all packages are included
func (f *Fissile) GeneratePackagesRoleImage(repository string, roleManifest *model.RoleManifest, noBuild, force bool, roles model.Roles, packagesImageBuilder *builder.PackagesImageBuilder) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	packagesLayerImageName, err := packagesImageBuilder.GetRolePackageImageName(roleManifest, roles)
	if err != nil {
		return fmt.Errorf("Error finding role's package name: %s", err.Error())
	}
	if !force {
		if hasImage, err := dockerManager.HasImage(packagesLayerImageName); err == nil && hasImage {
			f.UI.Printf("Packages layer %s already exists. Skipping ...\n", color.YellowString(packagesLayerImageName))
			return nil
		}
	}

	baseImageName := builder.GetBaseImageName(repository, f.Version)
	if hasImage, err := dockerManager.HasImage(baseImageName); err != nil {
		return fmt.Errorf("Error getting base image: %s", err)
	} else if !hasImage {
		return fmt.Errorf("Failed to find role base %s, did you build it first?", baseImageName)
	}

	if noBuild {
		f.UI.Println("Skipping packages layer docker image build because of --no-build flag.")
		return nil
	}

	f.UI.Printf("Building packages layer docker image %s ...\n",
		color.YellowString(packagesLayerImageName))
	log := new(bytes.Buffer)
	stdoutWriter := docker.NewFormattingWriter(
		log,
		docker.ColoredBuildStringFunc(packagesLayerImageName),
	)

	tarPopulator := packagesImageBuilder.NewDockerPopulator(roles, force)
	err = dockerManager.BuildImageFromCallback(packagesLayerImageName, stdoutWriter, tarPopulator)
	if err != nil {
		log.WriteTo(f.UI)
		return fmt.Errorf("Error building packages layer docker image: %s", err.Error())
	}
	f.UI.Println(color.GreenString("Done."))

	return nil
}

// GenerateRoleImages generates all role images using dev releases
func (f *Fissile) GenerateRoleImages(targetPath, repository, metricsPath string, noBuild, force bool, roleNames []string, workerCount int, rolesManifestPath, compiledPackagesPath, lightManifestPath, darkManifestPath string) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	if metricsPath != "" {
		stampy.Stamp(metricsPath, "fissile", "create-role-images", "start")
		defer stampy.Stamp(metricsPath, "fissile", "create-role-images", "done")
	}

	roleManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	opinions, err := model.NewOpinions(lightManifestPath, darkManifestPath)
	if err != nil {
		return err
	}
	if errs := f.validate(roleManifest, opinions); len(errs) != 0 {
		return fmt.Errorf(errs.Errors())
	}

	packagesImageBuilder, err := builder.NewPackagesImageBuilder(
		repository,
		compiledPackagesPath,
		targetPath,
		f.Version,
		f.UI,
	)
	if err != nil {
		return err
	}

	roles, err := roleManifest.SelectRoles(roleNames)
	if err != nil {
		return err
	}

	err = f.GeneratePackagesRoleImage(repository, roleManifest, noBuild, force, roles, packagesImageBuilder)
	if err != nil {
		return err
	}

	packagesLayerImageName, err := packagesImageBuilder.GetRolePackageImageName(roleManifest, roles)
	if err != nil {
		return err
	}

	roleBuilder, err := builder.NewRoleImageBuilder(
		repository,
		compiledPackagesPath,
		targetPath,
		lightManifestPath,
		darkManifestPath,
		metricsPath,
		"",
		f.Version,
		f.UI,
	)
	if err != nil {
		return err
	}

	if err := roleBuilder.BuildRoleImages(roles, repository, packagesLayerImageName, force, noBuild, workerCount); err != nil {
		return err
	}

	return nil
}

// ListRoleImages lists all dev role images
func (f *Fissile) ListRoleImages(repository string, rolesManifestPath string, existingOnDocker, withVirtualSize bool) error {
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
		devVersion, err := role.GetRoleDevVersion()
		if err != nil {
			return fmt.Errorf("Error creating role checksum: %s", err.Error())
		}

		imageName := builder.GetRoleDevImageName(repository, role, devVersion)

		if !existingOnDocker {
			f.UI.Println(imageName)
			continue
		}

		image, err := dockerManager.FindImage(imageName)

		if err == docker.ErrImageNotFound {
			continue
		} else if err != nil {
			return fmt.Errorf("Error looking up image: %s", err.Error())
		}

		if withVirtualSize {
			f.UI.Printf(
				"%s (%sMB)\n",
				color.GreenString(imageName),
				color.YellowString("%.2f", float64(image.VirtualSize)/(1024*1024)),
			)
		} else {
			f.UI.Println(imageName)
		}
	}

	return nil
}

//LoadReleases loads information about BOSH releases
func (f *Fissile) LoadReleases(releasePaths, releaseNames, releaseVersions []string, cacheDir string) error {
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
	err := f.injectPatchPropertiesJobSpec()
	if err != nil {
		return fmt.Errorf("Error loading release information: %s", err)
	}
	return nil
}

// DiffConfigurationBases generates a diff comparing the specs for two different BOSH releases
func (f *Fissile) DiffConfigurationBases(releasePaths []string, cacheDir string) error {
	hashDiffs, err := f.GetDiffConfigurationBases(releasePaths, cacheDir)
	if err != nil {
		return err
	}
	f.reportHashDiffs(hashDiffs)
	return nil
}

// GetDiffConfigurationBases calculates the difference in configs and returns a hash
func (f *Fissile) GetDiffConfigurationBases(releasePaths []string, cacheDir string) (*HashDiffs, error) {
	if len(releasePaths) != 2 {
		return nil, fmt.Errorf("expected two release paths, got %d", len(releasePaths))
	}
	defaultValues := []string{}
	err := f.LoadReleases(releasePaths, defaultValues, defaultValues, cacheDir)
	if err != nil {
		return nil, fmt.Errorf("dev config diff: error loading release information: %s", err)
	}
	return getDiffsFromReleases(f.releases)
}

// Since each job processes only its own spec, inject the designated
// patches-properties pseudo-job's spec into all the other jobs' specs.
func (f *Fissile) injectPatchPropertiesJobSpec() error {
	if f.patchPropertiesReleaseName == "" || f.patchPropertiesJobName == "" {
		return nil
	}
	var patchPropertiesJob *model.Job
	for _, release := range f.releases {
		if release.Name == f.patchPropertiesReleaseName {
			job, _ := release.LookupJob(f.patchPropertiesJobName)
			if job != nil {
				patchPropertiesJob = job
			}
		}
	}
	if patchPropertiesJob == nil {
		return nil
	}
	for _, release := range f.releases {
		for _, job := range release.Jobs {
			if release.Name != f.patchPropertiesReleaseName || job != patchPropertiesJob {
				job.MergeSpec(patchPropertiesJob)
			}
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

func (f *Fissile) reportHashDiffs(hashDiffs *HashDiffs) {
	if len(hashDiffs.DeletedKeys) > 0 {
		f.UI.Println(color.RedString("Dropped keys:"))
		sort.Strings(hashDiffs.DeletedKeys)
		for _, v := range hashDiffs.DeletedKeys {
			f.UI.Printf("  %s\n", v)
		}
	}
	if len(hashDiffs.AddedKeys) > 0 {
		f.UI.Println(color.GreenString("Added keys:"))
		sort.Strings(hashDiffs.AddedKeys)
		for _, v := range hashDiffs.AddedKeys {
			f.UI.Printf("  %s\n", v)
		}
	}
	if len(hashDiffs.ChangedValues) > 0 {
		f.UI.Println(color.BlueString("Changed values:"))
		sortedKeys := make([]string, len(hashDiffs.ChangedValues))
		i := 0
		for k := range hashDiffs.ChangedValues {
			sortedKeys[i] = k
			i++
		}
		sort.Strings(sortedKeys)
		for _, k := range sortedKeys {
			v := hashDiffs.ChangedValues[k]
			f.UI.Printf("  %s: %s => %s\n", k, v[0], v[1])
		}
	}
}

func getDiffsFromReleases(releases []*model.Release) (*HashDiffs, error) {
	hashes := [2]keyHash{keyHash{}, keyHash{}}
	for idx, release := range releases {
		configs := release.GetUniqueConfigs()
		for _, config := range configs {
			hashes[idx][config.Name] = config.Description
		}
		// Get the spec configs
		for _, job := range release.Jobs {
			for _, property := range job.Properties {
				key := fmt.Sprintf("%s.%s.%s", release.Name, job.Name, property.Name)
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

// GenerateKube will create a set of configuration files suitable for deployment
// on Kubernetes
func (f *Fissile) GenerateKube(rolesManifestPath, outputDir, repository, registry, organization string, defaultFiles []string, useMemoryLimits bool) error {

	rolesManifest, err := model.LoadRoleManifest(rolesManifestPath, f.releases)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	f.UI.Println("Loading defaults from env files")
	defaults, err := godotenv.Read(defaultFiles...)
	if err != nil {
		return err
	}

	settings := &kube.ExportSettings{
		Defaults:        defaults,
		Registry:        registry,
		Organization:    organization,
		Repository:      repository,
		UseMemoryLimits: useMemoryLimits,
	}

roleLoop:
	for _, role := range rolesManifest.Roles {
		for _, tag := range role.Tags {
			switch tag {
			case "dev-only":
				continue roleLoop
			}
		}

		roleTypeDir := filepath.Join(outputDir, string(role.Type))
		if err = os.MkdirAll(roleTypeDir, 0755); err != nil {
			return err
		}
		outputPath := filepath.Join(roleTypeDir, fmt.Sprintf("%s.yml", role.Name))

		f.UI.Printf("Writing config %s for role %s\n",
			color.CyanString(outputPath),
			color.CyanString(role.Name),
		)

		outputFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer outputFile.Close()

		switch role.Type {
		case model.RoleTypeBoshTask:
			job, err := kube.NewJob(role, settings)
			if err != nil {
				return err
			}

			if err := kube.WriteYamlConfig(job, outputFile); err != nil {
				return err
			}

		case model.RoleTypeBosh:
			needsStorage := len(role.Run.PersistentVolumes) != 0 || len(role.Run.SharedVolumes) != 0

			if role.HasTag("clustered") || needsStorage {
				statefulSet, deps, err := kube.NewStatefulSet(role, settings)
				if err != nil {
					return err
				}

				if err := kube.WriteYamlConfig(statefulSet, outputFile); err != nil {
					return err
				}

				if err := kube.WriteYamlConfig(deps, outputFile); err != nil {
					return err
				}

				continue
			}

			deployment, svc, err := kube.NewDeployment(role, settings)
			if err != nil {
				return err
			}

			if err := kube.WriteYamlConfig(deployment, outputFile); err != nil {
				return err
			}

			if svc != nil {
				if err := kube.WriteYamlConfig(svc, outputFile); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (f *Fissile) validate(roleManifest *model.RoleManifest, opinions *model.Opinions) validation.ErrorList {
	allErrs := validation.ErrorList{}

	bosh := f.collectPropertyDefaults()
	// map: property.name -> (default.string -> [*job...]

	dark := model.FlatMap(opinions.Dark)
	light := model.FlatMap(opinions.Light)
	// map[string]string

	properties := manifestProperties(roleManifest)
	// map[string]string

	// All properties must be defined in a BOSH release
	allErrs = append(allErrs, validateBosh("role-manifest",
		properties, bosh)...)

	// All light opinions must exists in a bosh release
	allErrs = append(allErrs, validateBosh("light opinion",
		light, bosh)...)

	// All dark opinions must exists in a bosh release
	allErrs = append(allErrs, validateBosh("dark opinion",
		dark, bosh)...)

	// All dark opinions must be configured as templates
	allErrs = append(allErrs, darkExposed(dark, properties)...)

	// No dark opinions must have defaults in light opinions
	allErrs = append(allErrs, darkUnexposed(dark, light)...)

	// No duplicates must exist between role manifest and light
	// opinions
	allErrs = append(allErrs, checkOverrides(light, roleManifest)...)

	// All bosh properties in a release should have the same
	// default across jobs -- WARNING only, not error
	f.checkBoshDefaults(bosh)

	// All light opinions should differ from their defaults in the
	// BOSH releases
	allErrs = append(allErrs, f.checkLightDefaults(light, bosh)...)

	// 	allErrs = append(allErrs, XXX()...)

	return allErrs
}

func validateBosh(label string, properties map[string]string, bosh propertyDefaults) validation.ErrorList {
	// All provided properties must be defined in a BOSH release
	allErrs := validation.ErrorList{}

	for property := range properties {
		// Ignore specials (without the "properties." prefix)
		if !strings.HasPrefix(property, "properties.") {
			continue
		}
		p := strings.TrimPrefix(property, "properties.")

		if _, ok := bosh[p]; !ok {
			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("%s '%s'", label, p), "In any BOSH release"))
		}
	}

	return allErrs
}

func manifestProperties(roleManifest *model.RoleManifest) map[string]string {
	properties := make(map[string]string)

	// Per-role properties
	for _, role := range roleManifest.Roles {
		for property, template := range role.Configuration.Templates {
			if _, ok := properties[property]; ok {
				continue
			}
			properties[property] = template
		}
	}

	// And the global properties
	for property, template := range roleManifest.Configuration.Templates {
		if _, ok := properties[property]; ok {
			continue
		}
		properties[property] = template
	}

	return properties
}

func darkExposed(dark map[string]string, properties map[string]string) validation.ErrorList {
	// All dark opinions must be configured as templates
	allErrs := validation.ErrorList{}

	for property := range dark {
		if _, ok := properties[property]; ok {
			continue
		}
		allErrs = append(allErrs, validation.NotFound(
			property, "Dark opinion is missing template in role-manifest"))
	}

	return allErrs
}

func darkUnexposed(dark map[string]string, light map[string]string) validation.ErrorList {
	// No dark opinions must have defaults in light opinions
	allErrs := validation.ErrorList{}

	for property := range dark {
		if _, ok := light[property]; !ok {
			continue
		}
		allErrs = append(allErrs, validation.Forbidden(
			property, "Dark opinion found in light opinions"))
	}

	return allErrs
}

func checkOverrides(light map[string]string, roleManifest *model.RoleManifest) validation.ErrorList {
	// No duplicates must exist between role manifest and light opinions
	allErrs := validation.ErrorList{}

	// Per-role properties
	for _, role := range roleManifest.Roles {
		for property, template := range role.Configuration.Templates {
			allErrs = append(allErrs, checkOverrideProperty(property, template, light, false)...)
		}
	}

	// And the global properties
	for property, template := range roleManifest.Configuration.Templates {
		allErrs = append(allErrs, checkOverrideProperty(property, template, light, true)...)
	}

	return allErrs
}

func checkOverrideProperty(property, value string, light map[string]string, conflicts bool) validation.ErrorList {
	allErrs := validation.ErrorList{}

	lightvalue, ok := light[property]
	if !ok {
		return allErrs
	}

	if lightvalue == value {
		return append(allErrs, validation.Forbidden(property,
			"Role-manifest duplicates opinion, remove from manifest"))
	}

	if conflicts {
		return append(allErrs, validation.Forbidden(property,
			"Role-manifest overrides opinion, remove opinion"))
	}

	return allErrs
}

func (f *Fissile) checkBoshDefaults(pd propertyDefaults) {
	// All bosh properties in a release should have the same
	// default across jobs

	// Ignore properties with a single default across all definitions.
	for property, defaults := range pd {
		if len(defaults) == 1 {
			continue
		}

		f.UI.Printf("%s: Property %s has %s defaults:\n",
			color.YellowString("Warning"),
			color.YellowString(property),
			color.YellowString(fmt.Sprintf("%d", len(defaults))))

		maxlen := 0
		for defaultv := range defaults {
			ds := fmt.Sprintf("%v", defaultv)
			if len(ds) > maxlen {
				maxlen = len(ds)
			}
		}

		leftjustified := fmt.Sprintf("%%-%ds", maxlen)

		for defaultv, jobs := range defaults {
			ds := fmt.Sprintf("%v", defaultv)
			if len(jobs) == 1 {
				job := jobs[0]
				f.UI.Printf("- Default %s: Release %s, job %s\n",
					color.CyanString(fmt.Sprintf(leftjustified, ds)),
					color.CyanString(job.Release.Name),
					color.CyanString(job.Name))
			} else {
				f.UI.Printf("- Default %s:\n", color.CyanString(ds))
				for _, job := range jobs {
					f.UI.Printf("  - Release %s, job %s\n",
						color.CyanString(job.Release.Name),
						color.CyanString(job.Name))
				}
			}
		}
	}
}

func (f *Fissile) checkLightDefaults(light map[string]string, pd propertyDefaults) validation.ErrorList {
	// All light opinions should differ from their defaults in the
	// BOSH releases

	// light :: (property.name -> value-of-opinion)
	// pd    :: (property.name -> (default.string -> [*job...])
	allErrs := validation.ErrorList{}

	for property, opinion := range light {
		// Ignore specials (without the "properties." prefix)
		if !strings.HasPrefix(property, "properties.") {
			continue
		}
		p := strings.TrimPrefix(property, "properties.")

		// Ignore unknown/undefined property
		defaults, ok := pd[p]
		if !ok {
			continue
		}

		// Ignore properties with ambigous defaults. Warn however.
		if len(defaults) > 1 {
			f.UI.Printf("light opinion %s ignored, %s\n",
				color.YellowString(p),
				color.YellowString("ambiguous default"))
			continue
		}

		// len(defaults) == 1 --> This loop will run only once
		// Is there a better (more direct?) way to get the
		// single key, i.e. default from the map ?
		for thedefault := range defaults {
			if opinion != thedefault {
				continue
			}
			allErrs = append(allErrs, validation.Forbidden(property,
				fmt.Sprintf("Light opinion matches default of '%v'",
					thedefault)))
		}
	}

	return allErrs
}
