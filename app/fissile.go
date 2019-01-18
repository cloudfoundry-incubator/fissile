package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"code.cloudfoundry.org/fissile/builder"
	"code.cloudfoundry.org/fissile/compilator"
	"code.cloudfoundry.org/fissile/docker"
	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/kube"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/loader"
	"code.cloudfoundry.org/fissile/model/releaseresolver"
	"code.cloudfoundry.org/fissile/scripts/compilation"
	"code.cloudfoundry.org/fissile/util"
	"github.com/SUSE/stampy"
	"github.com/SUSE/termui"
	"github.com/fatih/color"
	yaml "gopkg.in/yaml.v2"
)

// OutputFormat is one of the known output formats for commands showing loaded
// information.
type OutputFormat string

// Valid output formats
const (
	OutputFormatHuman = "human" // output for human consumption
	OutputFormatJSON  = "json"  // output as JSON
	OutputFormatYAML  = "yaml"  // output as YAML
)

// Fissile represents a fissile application.
type Fissile struct {
	Version   string
	UI        *termui.UI
	Manifest  *model.RoleManifest
	Options   FissileOptions
	cmdErr    error
	graphFile *os.File
}

// FissileOptions contains the values of all global fissile application options.
type FissileOptions struct {
	RoleManifest       string
	Releases           []string
	ReleaseNames       []string
	ReleaseVersions    []string
	FinalReleasesDir   string
	CacheDir           string
	WorkDir            string
	DockerRegistry     string
	DockerOrganization string
	DockerUsername     string
	DockerPassword     string
	RepositoryPrefix   string
	Workers            int
	LightOpinions      string
	DarkOpinions       string
	OutputFormat       string
	Metrics            string
	Verbose            bool
}

// NewFissileApplication creates a new app.Fissile.
func NewFissileApplication(version string, ui *termui.UI) *Fissile {
	return &Fissile{
		Version: version,
		UI:      ui,
	}
}

// Cleanup is a destructor.
func (f *Fissile) Cleanup() {
	f.GraphEnd()
}

// CompilationDir returns the path to the compilation directory.
func (f *Fissile) CompilationDir() string {
	return filepath.Join(f.Options.WorkDir, "compilation")
}

// StemcellCompilationDir returns the path to the compilation directory for a particular stemcell.
func (f *Fissile) StemcellCompilationDir(stemcell string) string {
	return filepath.Join(f.CompilationDir(), util.Hash(stemcell))
}

// LoadManifest loads the manifest in use by fissile.
func (f *Fissile) LoadManifest() error {
	roleManifest, err := loader.LoadRoleManifest(
		f.Options.RoleManifest,
		model.LoadRoleManifestOptions{
			ReleaseOptions: model.ReleaseOptions{
				ReleasePaths:     f.Options.Releases,
				ReleaseNames:     f.Options.ReleaseNames,
				ReleaseVersions:  f.Options.ReleaseVersions,
				BOSHCacheDir:     f.Options.CacheDir,
				FinalReleasesDir: f.Options.FinalReleasesDir,
			},
			Grapher: f,
		},
	)
	if err != nil {
		return fmt.Errorf("Error loading role manifest: %v", err)
	}

	f.Manifest = roleManifest
	return nil
}

// ListPackages will list all BOSH packages within a list of releases.
func (f *Fissile) ListPackages() error {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.Manifest.LoadedReleases {
		var releasePath string

		if f.Options.Verbose {
			releasePath = color.WhiteString(" (%s)", release.Path)
		}

		f.UI.Println(color.GreenString("%s release %s (%s)%s", release.ReleaseType(), color.YellowString(release.Name), color.MagentaString(release.Version), releasePath))

		for _, pkg := range release.Packages {
			var isCached string

			if f.Options.Verbose {
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

// ListJobs will list all jobs within a list of releases.
func (f *Fissile) ListJobs() error {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	for _, release := range f.Manifest.LoadedReleases {
		var releasePath string

		if f.Options.Verbose {
			releasePath = color.WhiteString(" (%s)", release.Path)
		}

		f.UI.Println(color.GreenString("%s release %s (%s)%s", release.ReleaseType(), color.YellowString(release.Name), color.MagentaString(release.Version), releasePath))

		for _, job := range release.Jobs {
			var isCached string

			if f.Options.Verbose {
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

// ListProperties will list all properties in all jobs within a list of releases.
func (f *Fissile) ListProperties() error {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	switch f.Options.OutputFormat {
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
		return fmt.Errorf("Invalid output format '%s', expected one of human, json, or yaml", f.Options.OutputFormat)
	}

	return nil
}

// SerializePackages returns all packages in loaded releases, keyed by fingerprint.
func (f *Fissile) SerializePackages() (map[string]interface{}, error) {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return nil, fmt.Errorf("Releases not loaded")
	}

	pkgs := make(map[string]interface{})
	for _, release := range f.Manifest.LoadedReleases {
		for _, pkg := range release.Packages {
			pkgs[pkg.Fingerprint] = util.NewMarshalAdapter(pkg)
		}
	}
	return pkgs, nil
}

// SerializeReleases will return all of the loaded releases.
func (f *Fissile) SerializeReleases() (map[string]interface{}, error) {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return nil, fmt.Errorf("Releases not loaded")
	}

	releases := make(map[string]interface{})
	for _, release := range f.Manifest.LoadedReleases {
		releases[release.Name] = util.NewMarshalAdapter(release)
	}
	return releases, nil
}

// SerializeJobs will return all of the jobs within the releases, keyed by fingerprint.
func (f *Fissile) SerializeJobs() (map[string]interface{}, error) {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return nil, fmt.Errorf("Releases not loaded")
	}

	jobs := make(map[string]interface{})
	for _, release := range f.Manifest.LoadedReleases {
		for _, job := range release.Jobs {
			jobs[job.Fingerprint] = util.NewMarshalAdapter(job)
		}
	}
	return jobs, nil
}

func (f *Fissile) listPropertiesForHuman() {
	// Human readable output.
	for _, release := range f.Manifest.LoadedReleases {
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

	for _, release := range f.Manifest.LoadedReleases {
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
// information (whether it is a hash, or not).

type propertyInfo struct {
	maybeHash bool
	defaults  map[string][]*model.Job
}

func (f *Fissile) collectPropertyDefaults() propertyDefaults {
	result := make(propertyDefaults)

	for _, release := range f.Manifest.LoadedReleases {
		for _, job := range release.Jobs {
			for _, property := range job.Properties {
				// Extend map for newly seen properties.
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
				// (uaa.clients, cc.quota_definitions).

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

// Compile will compile a list of dev BOSH releases.
func (f *Fissile) Compile(stemcellImageName string, targetPath, roleManifestPath, metricsPath string, instanceGroupNames, releaseNames []string, workerCount int, dockerNetworkMode string, withoutDocker, verbose bool, packageCacheConfigFilename string, streamPackages bool) error {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	if metricsPath != "" {
		stampy.Stamp(metricsPath, "fissile", "compile-packages", "start")
		defer stampy.Stamp(metricsPath, "fissile", "compile-packages", "done")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %v", err)
	}

	releases, err := f.getReleasesByName(releaseNames)
	if err != nil {
		return err
	}

	f.UI.Println(color.GreenString("Compiling packages for releases:"))
	for _, release := range releases {
		f.UI.Printf("         %s (%s)\n", color.YellowString(release.Name), color.MagentaString(release.Version))
	}

	packageStorage, err := compilator.NewPackageStorageFromConfig(packageCacheConfigFilename, targetPath, stemcellImageName)
	if err != nil {
		return err
	}
	var comp *compilator.Compilator
	if withoutDocker {
		comp, err = compilator.NewMountNSCompilator(targetPath, metricsPath, stemcellImageName, compilation.LinuxBase, f.Version, f.UI, f, packageStorage)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %v", err)
		}
	} else {
		comp, err = compilator.NewDockerCompilator(dockerManager, targetPath, metricsPath, stemcellImageName, compilation.LinuxBase, f.Version, dockerNetworkMode, false, f.UI, f, packageStorage, streamPackages)
		if err != nil {
			return fmt.Errorf("Error creating a new compilator: %v", err)
		}
	}

	instanceGroups, err := f.Manifest.SelectInstanceGroups(instanceGroupNames)
	if err != nil {
		return fmt.Errorf("Error selecting packages to build: %v", err)
	}

	if err := comp.Compile(workerCount, releases, instanceGroups, verbose); err != nil {
		return fmt.Errorf("Error compiling packages: %v", err)
	}

	return nil
}

// CleanCache inspects the compilation cache and removes all packages
// which are not referenced (anymore).
func (f *Fissile) CleanCache() error {
	targetPath := f.CompilationDir()
	// 1. Collect list of packages referenced by the releases. A
	//    variant of the code in ListPackages, we keep only the
	//    hashes.

	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	referenced := make(map[string]int)
	for _, release := range f.Manifest.LoadedReleases {
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

// ListRoleImages lists all dev role images.
func (f *Fissile) ListRoleImages(existingOnDocker, withVirtualSize bool, tagExtra string) error {
	if withVirtualSize && !existingOnDocker {
		return fmt.Errorf("Cannot list image virtual sizes if not matching image names with docker")
	}

	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	var dockerManager *docker.ImageManager
	var err error

	if existingOnDocker {
		dockerManager, err = docker.NewImageManager()
		if err != nil {
			return fmt.Errorf("Error connecting to docker: %v", err)
		}
	}

	opinions, err := model.NewOpinions(f.Options.LightOpinions, f.Options.DarkOpinions)
	if err != nil {
		return fmt.Errorf("Error loading opinions: %v", err)
	}

	for _, instanceGroup := range f.Manifest.InstanceGroups {
		devVersion, err := instanceGroup.GetRoleDevVersion(opinions, tagExtra, f.Version, f)
		if err != nil {
			return fmt.Errorf("Error creating instance group checksum: %v", err)
		}

		imageName := builder.GetRoleDevImageName(f.Options.DockerRegistry, f.Options.DockerOrganization, f.Options.RepositoryPrefix, instanceGroup, devVersion)

		if !existingOnDocker {
			f.UI.Println(imageName)
			continue
		}

		image, err := dockerManager.FindImage(imageName)

		if _, ok := err.(docker.ErrImageNotFound); ok {
			continue
		} else if err != nil {
			return fmt.Errorf("Error looking up image: %v", err)
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

// getReleasesByName returns all named releases, or all releases if no names are given.
func (f *Fissile) getReleasesByName(releaseNames []string) ([]*model.Release, error) {
	if len(releaseNames) == 0 {
		return f.Manifest.LoadedReleases, nil
	}

	var releases []*model.Release
	var missingReleases []string
releaseNameLoop:
	for _, releaseName := range releaseNames {
		for _, release := range f.Manifest.LoadedReleases {
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

// DiffConfigurationBases generates a diff comparing the specs for two different BOSH releases.
func (f *Fissile) DiffConfigurationBases(releasePaths []string, cacheDir string) error {
	hashDiffs, err := f.GetDiffConfigurationBases(releasePaths, cacheDir)
	if err != nil {
		return err
	}
	f.reportHashDiffs(hashDiffs)
	return nil
}

// GetDiffConfigurationBases calculates the difference in configs and returns a hash.
func (f *Fissile) GetDiffConfigurationBases(releasePaths []string, cacheDir string) (*HashDiffs, error) {
	if len(releasePaths) != 2 {
		return nil, fmt.Errorf("expected two release paths, got %d", len(releasePaths))
	}
	defaultValues := []string{}
	releases, err := releaseresolver.LoadReleasesFromDisk(model.ReleaseOptions{
		ReleasePaths:    releasePaths,
		ReleaseNames:    defaultValues,
		ReleaseVersions: defaultValues,
		BOSHCacheDir:    cacheDir})
	if err != nil {
		return nil, fmt.Errorf("dev config diff: error loading release information: %v", err)
	}
	return getDiffsFromReleases(releases)
}

type keyHash map[string]string

// HashDiffs summarizes the diffs between the two configs.
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
			f.UI.Printf("  %s:\n    %s\n    %s\n", k,
				strings.Replace(v[0], "\n", "\n    ", -1),
				strings.Replace(v[1], "\n", "\n    ", -1))
		}
	}
}

// stringifyValue returns a string representation of a value.
func stringifyValue(value reflect.Value) string {
	if value.Kind() == reflect.Interface && !value.IsNil() {
		// If the spec doesn't have a good idea of the type of the value (mostly
		// means children of hashes), we don't get a proper type here.  Look for
		// the underlying value instead.
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Invalid:
		return "<nil>"
	case reflect.Map:
		pairs := make([]string, 0, value.Len())
		for _, key := range value.MapKeys() {
			innerValue := stringifyValue(value.MapIndex(key))
			result := fmt.Sprintf("  %v:%s\n", key, strings.Replace(innerValue, "\n", "\n  ", -1))
			pairs = append(pairs, result)
		}
		sort.Strings(pairs)
		return fmt.Sprintf("map[\n%s]", strings.Join(pairs, ""))
	case reflect.Array, reflect.Slice:
		items := make([]string, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			innerValue := stringifyValue(value.Index(i))
			result := fmt.Sprintf("  %s\n", strings.Replace(innerValue, "\n", "\n  ", -1))
			items = append(items, result)
		}
		sort.Strings(items)
		return fmt.Sprintf("[\n%s]", strings.Join(items, ""))
	default:
		return fmt.Sprintf("%+v", value.Interface())
	}
}

func getDiffsFromReleases(releases []*model.Release) (*HashDiffs, error) {
	hashes := [2]keyHash{keyHash{}, keyHash{}}
	for idx, release := range releases {
		configs := release.GetUniqueConfigs()
		for _, config := range configs {
			hashes[idx][config.Name] = config.Description
		}
		// Get the spec configs.
		for _, job := range release.Jobs {
			for _, property := range job.Properties {
				key := fmt.Sprintf("%s:%s:%s", release.Name, job.Name, property.Name)
				hashes[idx][key] = stringifyValue(reflect.ValueOf(property.Default))
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
// on Kubernetes.
func (f *Fissile) GenerateKube(settings kube.ExportSettings) error {
	var err error
	settings.RoleManifest = f.Manifest

	cvs := model.MakeMapOfVariables(settings.RoleManifest)
	for key, value := range cvs {
		if !value.CVOptions.Secret {
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

	manifestSecret, err := kube.MakeBoshDeploymentManifestSecret(settings)
	if err != nil {
		return err
	}

	err = f.generateSecrets("deployment-manifest-secret.yaml", manifestSecret, settings)
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

	// Generate accounts (and any associated role bindings / cluster role bindings)
	for accountName, accountSpec := range settings.RoleManifest.Configuration.Authorization.Accounts {
		// Ignore accounts referenced by a single instance group. These are not
		// written as their own files, but as part of the instance group.
		if len(accountSpec.UsedBy) < 2 {
			continue
		}

		nodes, err := kube.NewRBACAccount(accountName, settings.RoleManifest.Configuration, settings)
		if err != nil {
			return err
		}
		err = f.writeHelmNode(authDir, fmt.Sprintf("account-%s.yaml", accountName), nodes...)
		if err != nil {
			return err
		}
	}

	// Generate roles
	for roleName, roleSpec := range settings.RoleManifest.Configuration.Authorization.Roles {
		var accountNames []string
		for accountName := range settings.RoleManifest.Configuration.Authorization.RoleUsedBy[roleName] {
			accountNames = append(accountNames, fmt.Sprintf("- %s", accountName))
		}
		if len(accountNames) < 1 {
			panic(fmt.Sprintf("Role \"%s\" used by no accounts", roleName))
		}
		if len(accountNames) < 2 {
			// Ignore roles referenced by a single account. These are not written as their own files,
			// but as part of the account.
			continue
		}
		sort.Strings(accountNames)

		node, err := kube.NewRBACRole(roleName, kube.RBACRoleKindRole, roleSpec, settings)
		if err != nil {
			return err
		}
		node.Set(helm.Comment(fmt.Sprintf("Role \"%s\" used by accounts:\n%s", roleName, strings.Join(accountNames, "\n"))))
		err = f.writeHelmNode(authDir, fmt.Sprintf("auth-role-%s.yaml", roleName), node)
		if err != nil {
			return err
		}
	}

	// Generate cluster roles
	for roleName, roleSpec := range settings.RoleManifest.Configuration.Authorization.ClusterRoles {
		var accountNames []string
		for accountName := range settings.RoleManifest.Configuration.Authorization.ClusterRoleUsedBy[roleName] {
			accountNames = append(accountNames, fmt.Sprintf("- %s", accountName))
		}
		if len(accountNames) < 1 {
			panic(fmt.Sprintf("Cluster role \"%s\" used by no accounts", roleName))
		}
		if len(accountNames) < 2 {
			// Ignore cluster roles referenced by a single account. These are not written as their own files,
			// but as part of the account.
			continue
		}
		sort.Strings(accountNames)

		node, err := kube.NewRBACRole(roleName, kube.RBACRoleKindClusterRole, roleSpec, settings)
		if err != nil {
			return err
		}
		node.Set(helm.Comment(fmt.Sprintf("Cluster role \"%s\" used by accounts:\n%s", roleName, strings.Join(accountNames, "\n"))))
		err = f.writeHelmNode(authDir, fmt.Sprintf("auth-cluster-role-%s.yaml", roleName), node)
		if err != nil {
			return err
		}
	}

	// Generate pod security policies
	for pspName, psp := range settings.RoleManifest.Configuration.Authorization.PodSecurityPolicies {
		// TODO: embed PSPs into instance group definitions as appropriate
		node, err := kube.NewRBACPSP(pspName, psp, settings)
		if err != nil {
			return err
		}
		err = f.writeHelmNode(authDir, fmt.Sprintf("auth-psp-%s.yaml", pspName), node)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *Fissile) writeHelmNode(dirName, fileName string, nodes ...helm.Node) error {
	outputPath := filepath.Join(dirName, fileName)
	f.UI.Printf("Writing config %s\n", color.CyanString(outputPath))

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		err = helm.NewEncoder(outputFile, helm.EmptyLines(true)).Encode(node)
		if err != nil {
			_ = outputFile.Close()
			return err
		}
	}
	err = outputFile.Close()
	return err
}

func (f *Fissile) generateBoshTaskRole(instanceGroup *model.InstanceGroup, settings kube.ExportSettings) ([]helm.Node, error) {

	var node helm.Node
	var err error

	if instanceGroup.HasTag(model.RoleTagStopOnFailure) {
		node, err = kube.NewPod(instanceGroup, settings, f)
	} else {
		node, err = kube.NewJob(instanceGroup, settings, f)
	}

	if err != nil {
		return nil, err
	}

	authNodes, err := f.generateAuthCoupledToRole(instanceGroup, settings)
	if err != nil {
		return nil, err
	}

	// We must generate the dependencies before the actual task, otherwise
	// `helm create -f` will get confused
	return append(authNodes, node), err
}

func (f *Fissile) generateAuthCoupledToRole(instanceGroup *model.InstanceGroup, settings kube.ExportSettings) ([]helm.Node, error) {
	accountName := instanceGroup.Run.ServiceAccount

	account := settings.RoleManifest.Configuration.Authorization.Accounts[accountName]
	if len(account.UsedBy) != 1 {
		// Account is used in multiple instance groups, don't embed it
		return nil, nil
	}
	if _, ok := account.UsedBy[instanceGroup.Name]; !ok {
		panic(fmt.Sprintf("Account %s is not used by instance group %s", accountName, instanceGroup.Name))
	}

	nodes, err := kube.NewRBACAccount(accountName, settings.RoleManifest.Configuration, settings)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// instanceGroupHasStorage returns true if a given group uses shared or
// persistent volumes.
func (f *Fissile) instanceGroupHasStorage(instanceGroup *model.InstanceGroup) bool {
	for _, volume := range instanceGroup.Run.Volumes {
		switch volume.Type {
		case model.VolumeTypePersistent, model.VolumeTypeShared:
			return true
		}
	}
	return false
}

func (f *Fissile) generateKubeRoles(settings kube.ExportSettings) error {
	for _, instanceGroup := range settings.RoleManifest.InstanceGroups {
		if instanceGroup.IsColocated() {
			continue
		}
		if settings.CreateHelmChart && instanceGroup.Run.FlightStage == model.FlightStageManual {
			continue
		}

		subDir := string(instanceGroup.Type)
		if settings.CreateHelmChart {
			subDir = "templates"
		}
		roleTypeDir := filepath.Join(settings.OutputDir, subDir)
		err := os.MkdirAll(roleTypeDir, 0755)
		if err != nil {
			return err
		}

		switch instanceGroup.Type {
		case model.RoleTypeBoshTask:
			nodes, err := f.generateBoshTaskRole(instanceGroup, settings)
			if err != nil {
				return err
			}

			err = f.writeHelmNode(roleTypeDir, fmt.Sprintf("%s.yaml", instanceGroup.Name), nodes...)
			if err != nil {
				return err
			}

		case model.RoleTypeBosh:
			statefulSet, deps, err := kube.NewStatefulSet(instanceGroup, settings, f)
			if err != nil {
				return err
			}

			authNodes, err := f.generateAuthCoupledToRole(instanceGroup, settings)
			if err != nil {
				return err
			}

			nodes := authNodes
			if deps != nil {
				nodes = append(nodes, deps)
			}
			nodes = append(nodes, statefulSet)

			err = f.writeHelmNode(roleTypeDir, fmt.Sprintf("%s.yaml", instanceGroup.Name), nodes...)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GraphBegin will start logging hash information to the given file.
func (f *Fissile) GraphBegin(outputPath string) error {
	if outputPath == "" {
		return nil
	}
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

// GraphEnd will stop logging hash information.
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

// GraphNode adds a node to the hash debugging graph; this implements model.ModelGrapher.
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

// GraphEdge adds an edge to the hash debugging graph; this implements model.ModelGrapher.
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
