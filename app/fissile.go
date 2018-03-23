package app

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/SUSE/fissile/builder"
	"github.com/SUSE/fissile/compilator"
	"github.com/SUSE/fissile/docker"
	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/kube"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/scripts/compilation"
	"github.com/SUSE/fissile/util"
	"github.com/SUSE/stampy"
	"github.com/SUSE/termui"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

// OutputFormat is one of the known output formats for commands showing loaded
// information
type OutputFormat string

// Valid output formats
const (
	OutputFormatHuman = "human" // output for human consumption
	OutputFormatJSON  = "json"  // output as JSON
	OutputFormatYAML  = "yaml"  // output as YAML
)

// Fissile represents a fissile application
type Fissile struct {
	Version   string
	UI        *termui.UI
	cmdErr    error
	releases  []*model.Release // Only applies for some commands
	graphFile *os.File
}

// NewFissileApplication creates a new app.Fissile
func NewFissileApplication(version string, ui *termui.UI) *Fissile {
	return &Fissile{
		Version: version,
		UI:      ui,
	}
}

// ListPackages will list all BOSH packages within a list of releases
func (f *Fissile) ListPackages(verbose bool) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.releases {
		var releasePath string

		if verbose {
			releasePath = color.WhiteString(" (%s)", release.Path)
		}

		f.UI.Println(color.GreenString("%s release %s (%s)%s", release.ReleaseType(), color.YellowString(release.Name), color.MagentaString(release.Version), releasePath))

		for _, pkg := range release.Packages {
			var isCached string

			if verbose {
				if _, err := os.Stat(pkg.Path); err == nil {
					isCached = color.WhiteString(" (cached at %s)", pkg.Path)
				} else {
					isCached = color.RedString(" (cache not found)")
				}
			}

			f.UI.Printf("%s (%s)%s\n", color.YellowString(pkg.Name), color.WhiteString(pkg.Version), isCached)
		}

		f.UI.Printf(
			"There are %s packages present.\n\n",
			color.GreenString("%d", len(release.Packages)),
		)
	}

	return nil
}

// ListJobs will list all jobs within a list of releases
func (f *Fissile) ListJobs(verbose bool) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.releases {
		var releasePath string

		if verbose {
			releasePath = color.WhiteString(" (%s)", release.Path)
		}

		f.UI.Println(color.GreenString("%s release %s (%s)%s", release.ReleaseType(), color.YellowString(release.Name), color.MagentaString(release.Version), releasePath))

		for _, job := range release.Jobs {
			var isCached string

			if verbose {
				if _, err := os.Stat(job.Path); err == nil {
					isCached = color.WhiteString(" (cached at %s)", job.Path)
				} else {
					isCached = color.RedString(" (cache not found)")
				}
			}

			f.UI.Printf("%s (%s)%s: %s\n", color.YellowString(job.Name), color.WhiteString(job.Version), isCached, job.Description)
		}

		f.UI.Printf(
			"There are %s jobs present.\n\n",
			color.GreenString("%d", len(release.Jobs)),
		)
	}

	return nil
}

// ListProperties will list all properties in all jobs within a list of releases
func (f *Fissile) ListProperties(outputFormat OutputFormat) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	switch outputFormat {
	case OutputFormatHuman:
		f.listPropertiesForHuman()
	case OutputFormatJSON:
		// Note: The encoding/json is unable to handle type
		// -- map[interface {}]interface {}
		// Such types can occur when the default value has sub-structure.

		buf, err := util.JSONMarshal(f.collectProperties())
		if err != nil {
			return err
		}

		f.UI.Printf("%s", buf)
	case OutputFormatYAML:
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

// SerializePackages returns all packages in loaded releases, keyed by fingerprint
func (f *Fissile) SerializePackages() (map[string]interface{}, error) {
	if len(f.releases) == 0 {
		return nil, fmt.Errorf("Releases not loaded")
	}

	pkgs := make(map[string]interface{})
	for _, release := range f.releases {
		for _, pkg := range release.Packages {
			pkgs[pkg.Fingerprint] = util.NewMarshalAdapter(pkg)
		}
	}
	return pkgs, nil
}

// SerializeReleases will return all of the loaded releases
func (f *Fissile) SerializeReleases() (map[string]interface{}, error) {
	if len(f.releases) == 0 {
		return nil, fmt.Errorf("Releases not loaded")
	}

	releases := make(map[string]interface{})
	for _, release := range f.releases {
		releases[release.Name] = util.NewMarshalAdapter(release)
	}
	return releases, nil
}

// SerializeJobs will return all of the jobs within the releases, keyed by fingerprint
func (f *Fissile) SerializeJobs() (map[string]interface{}, error) {
	if len(f.releases) == 0 {
		return nil, fmt.Errorf("Releases not loaded")
	}

	jobs := make(map[string]interface{})
	for _, release := range f.releases {
		for _, job := range release.Jobs {
			jobs[job.Fingerprint] = util.NewMarshalAdapter(job)
		}
	}
	return jobs, nil
}

func (f *Fissile) listPropertiesForHuman() {
	// Human readable output.
	for _, release := range f.releases {
		f.UI.Println(color.GreenString("%s release %s (%s)",
			release.ReleaseType(), color.YellowString(release.Name), color.MagentaString(release.Version)))

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

// propertyDefaults is a map from property names to information about
// it needed for validation.
type propertyDefaults map[string]*propertyInfo

// propertyInfo is a structure listing the (stringified) defaults and
// the associated jobs for a property, plus other aggregated
// information (whether it is a hash, or not)

type propertyInfo struct {
	maybeHash bool
	defaults  map[string][]*model.Job
}

func (f *Fissile) collectPropertyDefaults() propertyDefaults {
	result := make(propertyDefaults)

	for _, release := range f.releases {
		for _, job := range release.Jobs {
			for _, property := range job.Properties {

				// Extend map for newly seen properties
				if _, ok := result[property.Name]; !ok {
					result[property.Name] = newPropertyInfo(false)
				}

				// Extend the map of defaults to job lists.
				defaultAsString := fmt.Sprintf("%v", property.Default)
				result[property.Name].defaults[defaultAsString] =
					append(result[property.Name].defaults[defaultAsString], job)

				// Handle the property's hash flag, based on the current default for
				// it. Note that if the default is <nil> we assume that it can be a
				// hash. This works arounds problems in the CF spec files where the two
				// hash-valued properties we are interested in do not have defaults.
				// (uaa.clients, cc.quota_definitions)

				if property.Default == nil ||
					reflect.TypeOf(property.Default).Kind() == reflect.Map {
					result[property.Name].maybeHash = true
				}
			}
		}
	}

	return result
}

// newPropertyInfo creates a new information block for a property.
func newPropertyInfo(maybeHash bool) *propertyInfo {
	return &propertyInfo{
		defaults:  make(map[string][]*model.Job),
		maybeHash: maybeHash,
	}
}

// Compile will compile a list of dev BOSH releases
func (f *Fissile) Compile(stemcellImageName string, targetPath, roleManifestPath, metricsPath string, roleNames, releaseNames []string, workerCount int, dockerNetworkMode string, withoutDocker, verbose bool) error {
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

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases, f)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	releases, err := f.getReleasesByName(releaseNames)
	if err != nil {
		return err
	}

	f.UI.Println(color.GreenString("Compiling packages for releases:"))
	for _, release := range releases {
		f.UI.Printf("         %s (%s)\n", color.YellowString(release.Name), color.MagentaString(release.Version))
	}

	var comp *compilator.Compilator
	if withoutDocker {
		comp, err = compilator.NewMountNSCompilator(targetPath, metricsPath, stemcellImageName, compilation.LinuxBase, f.Version, f.UI, f)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %s", err.Error())
		}
	} else {
		comp, err = compilator.NewDockerCompilator(dockerManager, targetPath, metricsPath, stemcellImageName, compilation.LinuxBase, f.Version, dockerNetworkMode, false, f.UI, f)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %s", err.Error())
		}
	}

	roles, err := roleManifest.SelectRoles(roleNames)
	if err != nil {
		return fmt.Errorf("Error selecting packages to build: %s", err.Error())
	}

	if err := comp.Compile(workerCount, releases, roles, verbose); err != nil {
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

	cached, err := filepath.Glob(targetPath + "/*/*")
	if err != nil {
		return err
	}

	removed := 0
	for _, cache := range cached {
		key := filepath.Base(cache)
		if _, ok := referenced[key]; ok {
			continue
		}

		relpath, err := filepath.Rel(targetPath, cache)
		if err != nil {
			relpath = key
		}
		f.UI.Printf("- Removing %s\n", color.YellowString(relpath))
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
func (f *Fissile) GeneratePackagesRoleImage(stemcellImageName string, roleManifest *model.RoleManifest, noBuild, force bool, roles model.Roles, packagesImageBuilder *builder.PackagesImageBuilder) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	packagesLayerImageName, err := packagesImageBuilder.GetPackagesLayerImageName(roleManifest, roles, f)
	if err != nil {
		return fmt.Errorf("Error finding role's package name: %s", err.Error())
	}
	if !force {
		if hasImage, err := dockerManager.HasImage(packagesLayerImageName); err == nil && hasImage {
			f.UI.Printf("Packages layer %s already exists. Skipping ...\n", color.YellowString(packagesLayerImageName))
			return nil
		}
	}

	if hasImage, err := dockerManager.HasImage(stemcellImageName); err != nil {
		return fmt.Errorf("Error looking up stemcell image: %s", err)
	} else if !hasImage {
		return fmt.Errorf("Failed to find stemcell image %s. Did you pull it?", stemcellImageName)
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

// GeneratePackagesRoleTarball builds a tarball snapshot of the build context
// for the docker image for the packages layer where all packages are included
func (f *Fissile) GeneratePackagesRoleTarball(repository string, roleManifest *model.RoleManifest, noBuild, force bool, roles model.Roles, outputDirectory string, packagesImageBuilder *builder.PackagesImageBuilder) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	packagesLayerImageName, err := packagesImageBuilder.GetPackagesLayerImageName(roleManifest, roles, f)
	if err != nil {
		return fmt.Errorf("Error finding role's package name: %v", err)
	}
	outputPath := filepath.Join(outputDirectory, fmt.Sprintf("%s.tar", packagesLayerImageName))

	if !force {
		info, err := os.Stat(outputPath)
		if err == nil && !info.IsDir() {
			f.UI.Printf("Packages layer %s already exists. Skipping ...\n", color.YellowString(outputPath))
			return nil
		}
	}

	if noBuild {
		f.UI.Println("Skipping packages layer tarball build because of --no-build flag.")
		return nil
	}

	f.UI.Printf("Building packages layer tarball %s ...\n", color.YellowString(outputPath))

	tarFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("Failed to create tar file %s: %s", outputPath, err)
	}
	tarWriter := tar.NewWriter(tarFile)

	// We always force build all packages here to avoid needing to talk to the
	// docker daemon to figure out what we can keep
	tarPopulator := packagesImageBuilder.NewDockerPopulator(roles, true)
	err = tarPopulator(tarWriter)
	if err != nil {
		return fmt.Errorf("Error writing tar file: %s", err)
	}
	err = tarWriter.Close()
	if err != nil {
		return fmt.Errorf("Error closing tar file: %s", err)
	}
	f.UI.Println(color.GreenString("Done."))

	return nil
}

// GenerateRoleImages generates all role images using releases
func (f *Fissile) GenerateRoleImages(targetPath, registry, organization, repository, stemcellImageName, stemcellImageID, metricsPath string, noBuild, force bool, tagExtra string, roleNames []string, workerCount int, roleManifestPath, compiledPackagesPath, lightManifestPath, darkManifestPath, outputDirectory string) error {
	if len(f.releases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	if metricsPath != "" {
		stampy.Stamp(metricsPath, "fissile", "create-role-images", "start")
		defer stampy.Stamp(metricsPath, "fissile", "create-role-images", "done")
	}

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases, f)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	opinions, err := model.NewOpinions(lightManifestPath, darkManifestPath)
	if err != nil {
		return err
	}
	if errs := f.validateManifestAndOpinions(roleManifest, opinions); len(errs) != 0 {
		return fmt.Errorf(errs.Errors())
	}

	if outputDirectory != "" {
		err = os.MkdirAll(outputDirectory, 0755)
		if err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("Output directory %s exists and is not a directory", outputDirectory)
			}
			if err != nil {
				return fmt.Errorf("Error creating directory %s: %s", outputDirectory, err)
			}
		}
	}

	packagesImageBuilder, err := builder.NewPackagesImageBuilder(
		repository,
		stemcellImageName,
		stemcellImageID,
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

	if outputDirectory == "" {
		err = f.GeneratePackagesRoleImage(stemcellImageName, roleManifest, noBuild, force, roles, packagesImageBuilder)
	} else {
		err = f.GeneratePackagesRoleTarball(stemcellImageName, roleManifest, noBuild, force, roles, outputDirectory, packagesImageBuilder)
	}
	if err != nil {
		return err
	}

	packagesLayerImageName, err := packagesImageBuilder.GetPackagesLayerImageName(roleManifest, roles, f)
	if err != nil {
		return err
	}

	roleBuilder, err := builder.NewRoleImageBuilder(
		stemcellImageName,
		compiledPackagesPath,
		targetPath,
		lightManifestPath,
		darkManifestPath,
		metricsPath,
		tagExtra,
		f.Version,
		f.UI,
		f,
	)
	if err != nil {
		return err
	}

	return roleBuilder.BuildRoleImages(roles, registry, organization, repository, packagesLayerImageName, outputDirectory, force, noBuild, workerCount)
}

// ListRoleImages lists all dev role images
func (f *Fissile) ListRoleImages(registry, organization, repository, roleManifestPath, opinionsPath, darkOpinionsPath string, existingOnDocker, withVirtualSize bool, tagExtra string) error {
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

	roleManifest, err := model.LoadRoleManifest(roleManifestPath, f.releases, f)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	opinions, err := model.NewOpinions(opinionsPath, darkOpinionsPath)
	if err != nil {
		return fmt.Errorf("Error loading opinions: %s", err.Error())
	}

	for _, role := range roleManifest.Roles {
		devVersion, err := role.GetRoleDevVersion(opinions, tagExtra, f.Version, f)
		if err != nil {
			return fmt.Errorf("Error creating role checksum: %s", err.Error())
		}

		imageName := builder.GetRoleDevImageName(registry, organization, repository, role, devVersion)

		if !existingOnDocker {
			f.UI.Println(imageName)
			continue
		}

		image, err := dockerManager.FindImage(imageName)

		if _, ok := err.(docker.ErrImageNotFound); ok {
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

		var release *model.Release
		var err error
		if _, err = isFinalReleasePath(releasePath); err == nil {
			// For final releases, only can use release name and version defined in release.MF, cannot specify them through flags.
			release, err = model.NewFinalRelease(releasePath)
			if err != nil {
				return fmt.Errorf("Error loading final release information: %s", err.Error())
			}
		} else {
			release, err = model.NewDevRelease(releasePath, releaseName, releaseVersion, cacheDir)
			if err != nil {
				return fmt.Errorf("Error loading dev release information: %s", err.Error())
			}
		}

		releases[idx] = release
	}

	f.releases = releases

	return nil
}

func isFinalReleasePath(releasePath string) (bool, error) {
	if err := util.ValidatePath(releasePath, true, "release directory"); err != nil {
		return false, err
	}

	if err := util.ValidatePath(filepath.Join(releasePath, "release.MF"), false, "release 'release.MF' file"); err != nil {
		return false, err
	}

	if err := util.ValidatePath(filepath.Join(releasePath, "dev_releases"), true, "release 'dev_releases' file"); err == nil {
		return false, err
	}

	if err := util.ValidatePath(filepath.Join(releasePath, "jobs"), true, "release 'jobs' directory"); err != nil {
		return false, err
	}

	if err := util.ValidatePath(filepath.Join(releasePath, "packages"), true, "release 'packages' directory"); err != nil {
		return false, err
	}

	return true, nil
}

// getReleasesByName returns all named releases, or all releases if no names are given
func (f *Fissile) getReleasesByName(releaseNames []string) ([]*model.Release, error) {
	if len(releaseNames) == 0 {
		return f.releases, nil
	}

	var releases []*model.Release
	var missingReleases []string
releaseNameLoop:
	for _, releaseName := range releaseNames {
		for _, release := range f.releases {
			if release.Name == releaseName {
				releases = append(releases, release)
				continue releaseNameLoop
			}
		}
		missingReleases = append(missingReleases, releaseName)
	}
	if len(missingReleases) > 0 {
		return nil, fmt.Errorf("Some releases are unknown: %v", missingReleases)
	}
	return releases, nil

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
func (f *Fissile) GenerateKube(roleManifestPath string, defaultFiles []string, settings kube.ExportSettings) error {
	var err error
	settings.RoleManifest, err = model.LoadRoleManifest(roleManifestPath, f.releases, f)
	if err != nil {
		return fmt.Errorf("Error loading roles manifest: %s", err.Error())
	}

	if len(defaultFiles) > 0 {
		f.UI.Println("Loading defaults from env files")
		settings.Defaults, err = godotenv.Read(defaultFiles...)
		if err != nil {
			return err
		}
	}

	cvs := model.MakeMapOfVariables(settings.RoleManifest)
	for key, value := range cvs {
		if !value.Secret {
			delete(cvs, key)
		}
	}
	// cvs now holds only the secrets.
	var secrets helm.Node
	secrets, err = kube.MakeSecrets(cvs, settings)
	if err != nil {
		return err
	}

	err = f.generateSecrets("secrets.yaml", secrets, settings)
	if err != nil {
		return err
	}

	registryCredentials, err := kube.MakeRegistryCredentials(settings)
	if err != nil {
		return err
	}

	err = f.generateSecrets("registry-secret.yaml", registryCredentials, settings)
	if err != nil {
		return err
	}

	err = f.generateAuth(settings)
	if err != nil {
		return err
	}

	if settings.CreateHelmChart {
		values, err := kube.MakeValues(settings)
		if err != nil {
			return err
		}
		err = f.writeHelmNode(settings.OutputDir, "values.yaml", values)
		if err != nil {
			return err
		}
	}

	return f.generateKubeRoles(settings)
}

func (f *Fissile) generateSecrets(fileName string, secrets helm.Node, settings kube.ExportSettings) error {
	subDir := "secrets"
	if settings.CreateHelmChart {
		subDir = "templates"
	}
	secretsDir := filepath.Join(settings.OutputDir, subDir)
	err := os.MkdirAll(secretsDir, 0755)
	if err != nil {
		return err
	}
	return f.writeHelmNode(secretsDir, fileName, secrets)
}

func (f *Fissile) generateAuth(settings kube.ExportSettings) error {
	subDir := "auth"
	if settings.CreateHelmChart {
		subDir = "templates"
	}
	authDir := filepath.Join(settings.OutputDir, subDir)
	err := os.MkdirAll(authDir, 0755)
	if err != nil {
		return err
	}
	for roleName, roleSpec := range settings.RoleManifest.Configuration.Authorization.Roles {
		node, err := kube.NewRBACRole(roleName, roleSpec, settings)
		if err != nil {
			return err
		}
		err = f.writeHelmNode(authDir, fmt.Sprintf("auth-role-%s.yaml", roleName), node)
		if err != nil {
			return err
		}
	}
	for accountName, accountSpec := range settings.RoleManifest.Configuration.Authorization.Accounts {
		nodes, err := kube.NewRBACAccount(accountName, accountSpec, settings)
		if err != nil {
			return err
		}
		outputPath := filepath.Join(authDir, fmt.Sprintf("account-%s.yaml", accountName))
		outputFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer outputFile.Close()
		encoder := helm.NewEncoder(outputFile, helm.EmptyLines(true))
		for _, n := range nodes {
			err = encoder.Encode(n)
			if err != nil {
				return err
			}
		}
		err = outputFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Fissile) writeHelmNode(dirName, fileName string, node helm.Node) error {
	outputPath := filepath.Join(dirName, fileName)
	f.UI.Printf("Writing config %s\n", color.CyanString(outputPath))

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}

	err = helm.NewEncoder(outputFile, helm.EmptyLines(true)).Encode(node)
	if err == nil {
		return outputFile.Close()
	}
	_ = outputFile.Close()
	return err
}

func (f *Fissile) generateBoshTaskRole(outputFile *os.File, role *model.Role, settings kube.ExportSettings) error {
	if role.IsStopOnFailureRole() {
		pod, err := kube.NewPod(role, settings, f)
		if err != nil {
			return err
		}
		err = helm.NewEncoder(outputFile).Encode(pod)
		if err != nil {
			return err
		}
	} else {
		job, err := kube.NewJob(role, settings, f)
		if err != nil {
			return err
		}
		err = helm.NewEncoder(outputFile).Encode(job)
		if err != nil {
			return err
		}
	}

	return nil
}

// roleIsStateful returns true if a given role needs to be a StatefulSet
func (f *Fissile) roleIsStateful(role *model.Role) bool {
	if role.HasTag("clustered") || role.HasTag("indexed") {
		return true
	}
	for _, volume := range role.Run.Volumes {
		switch volume.Type {
		case model.VolumeTypePersistent, model.VolumeTypeShared:
			return true
		}
	}
	return false
}

func (f *Fissile) generateKubeRoles(settings kube.ExportSettings) error {
	for _, role := range settings.RoleManifest.Roles {
		if role.IsDevRole() {
			continue
		}
		if settings.CreateHelmChart && role.Run.FlightStage == model.FlightStageManual {
			continue
		}

		subDir := string(role.Type)
		if settings.CreateHelmChart {
			subDir = "templates"
		}
		roleTypeDir := filepath.Join(settings.OutputDir, subDir)
		err := os.MkdirAll(roleTypeDir, 0755)
		if err != nil {
			return err
		}
		outputPath := filepath.Join(roleTypeDir, fmt.Sprintf("%s.yaml", role.Name))

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
			err := f.generateBoshTaskRole(outputFile, role, settings)
			if err != nil {
				return err
			}

		case model.RoleTypeBosh:
			enc := helm.NewEncoder(outputFile)

			if f.roleIsStateful(role) {
				statefulSet, deps, err := kube.NewStatefulSet(role, settings, f)
				if err != nil {
					return err
				}
				err = enc.Encode(statefulSet)
				if err != nil {
					return err
				}
				if deps != nil {
					err = enc.Encode(deps)
					if err != nil {
						return err
					}
				}

				continue
			}
			deployment, svc, err := kube.NewDeployment(role, settings, f)
			if err != nil {
				return err
			}
			err = enc.Encode(deployment)
			if err != nil {
				return err
			}
			if svc != nil {
				err = enc.Encode(svc)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// GraphBegin will start logging hash information to the given file
func (f *Fissile) GraphBegin(outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	_, err = file.WriteString("strict digraph {\n")
	if err != nil {
		return err
	}
	_, err = file.WriteString("graph[K=5]\n")
	if err != nil {
		return err
	}
	f.graphFile = file
	return nil
}

// GraphEnd will stop logging hash information
func (f *Fissile) GraphEnd() error {
	if f.graphFile == nil {
		return nil
	}
	if _, err := f.graphFile.WriteString("}\n"); err != nil {
		return err
	}
	if err := f.graphFile.Close(); err != nil {
		return err
	}
	f.graphFile = nil
	return nil
}

// GraphNode adds a node to the hash debugging graph; this implements model.ModelGrapher
func (f *Fissile) GraphNode(nodeName string, attrs map[string]string) error {
	if f.graphFile == nil {
		return nil
	}
	var attrString string
	for k, v := range attrs {
		attrString += fmt.Sprintf("[%s=\"%s\"]", k, v)
	}
	if _, err := fmt.Fprintf(f.graphFile, "\"%s\" %s\n", nodeName, attrString); err != nil {
		return err
	}
	return nil
}

// GraphEdge adds an edge to the hash debugging graph; this implements model.ModelGrapher
func (f *Fissile) GraphEdge(fromNode, toNode string, attrs map[string]string) error {
	if f.graphFile == nil {
		return nil
	}
	var attrString string
	for k, v := range attrs {
		attrString += fmt.Sprintf("[%s=\"%s\"]", k, v)
	}
	if _, err := fmt.Fprintf(f.graphFile, "\"%s\" -> \"%s\" %s\n", fromNode, toNode, attrString); err != nil {
		return err
	}
	return nil
}
