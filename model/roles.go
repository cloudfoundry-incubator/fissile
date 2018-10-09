package model

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"code.cloudfoundry.org/fissile/util"
	"code.cloudfoundry.org/fissile/validation"

	"github.com/hashicorp/go-multierror"
	"github.com/mholt/archiver"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
	"gopkg.in/yaml.v2"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	InstanceGroups InstanceGroups `yaml:"instance_groups"`
	Configuration  *Configuration `yaml:"configuration"`
	Variables      Variables
	Releases       []*ReleaseRef `yaml:"releases"`

	LoadedReleases   []*Release
	manifestFilePath string

	validationOptions RoleManifestValidationOptions
}

// RoleManifestValidationOptions allows tests to skip some parts of validation
type RoleManifestValidationOptions struct {
	AllowMissingScripts bool
}

type releaseByName map[string]*Release

// LoadRoleManifestOptions provides the input to LoadRoleManifest()
type LoadRoleManifestOptions struct {
	ReleasePaths      []string
	ReleaseNames      []string
	ReleaseVersions   []string
	BOSHCacheDir      string
	Grapher           util.ModelGrapher
	ValidationOptions RoleManifestValidationOptions
}

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, options LoadRoleManifestOptions) (*RoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	roleManifest := RoleManifest{validationOptions: options.ValidationOptions}
	roleManifest.manifestFilePath = manifestFilePath
	if err := yaml.Unmarshal(manifestContents, &roleManifest); err != nil {
		return nil, err
	}

	releases, err := LoadReleases(
		options.ReleasePaths,
		options.ReleaseNames,
		options.ReleaseVersions,
		options.BOSHCacheDir)
	if err != nil {
		return nil, err
	}

	embeddedReleases, err := roleManifest.loadReleaseReferences()
	if err != nil {
		return nil, err
	}

	roleManifest.LoadedReleases = append(releases, embeddedReleases...)
	if err != nil {
		return nil, err
	}

	if roleManifest.Configuration == nil {
		roleManifest.Configuration = &Configuration{}
	}

	if roleManifest.Configuration.Templates == nil {
		roleManifest.Configuration.Templates = yaml.MapSlice{}
	}

	// Parse CVOptions
	var definitions internalVariableDefinitions
	err = yaml.Unmarshal(manifestContents, &definitions)
	if err != nil {
		return nil, err
	}

	for i, v := range definitions.Variables {
		roleManifest.Variables[i].CVOptions = v.CVOptions
	}

	err = roleManifest.resolveRoleManifest(options.Grapher)
	if err != nil {
		return nil, err
	}
	return &roleManifest, nil
}

//LoadReleases loads information about BOSH releases
func LoadReleases(releasePaths, releaseNames, releaseVersions []string, cacheDir string) ([]*Release, error) {
	releases := make([]*Release, len(releasePaths))
	for idx, releasePath := range releasePaths {
		var releaseName, releaseVersion string
		if len(releaseNames) != 0 {
			releaseName = releaseNames[idx]
		}
		if len(releaseVersions) != 0 {
			releaseVersion = releaseVersions[idx]
		}
		var release *Release
		var err error
		if _, err = isFinalReleasePath(releasePath); err == nil {
			// For final releases, only can use release name and version defined in release.MF, cannot specify them through flags.
			release, err = NewFinalRelease(releasePath)
			if err != nil {
				return nil, fmt.Errorf("Error loading final release information: %s", err.Error())
			}
		} else {
			release, err = NewDevRelease(releasePath, releaseName, releaseVersion, cacheDir)
			if err != nil {
				return nil, fmt.Errorf("Error loading dev release information: %s", err.Error())
			}
		}
		releases[idx] = release
	}
	return releases, nil
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

// loadReleaseReferences downloads/builds and loads releases referenced in the
// manifest
func (m *RoleManifest) loadReleaseReferences() ([]*Release, error) {
	releases := []*Release{}

	var allErrs error
	var wg sync.WaitGroup
	progress := mpb.New(mpb.WithWaitGroup(&wg))

	// go through each referenced release
	for _, releaseRef := range m.Releases {
		wg.Add(1)

		go func(releaseRef *ReleaseRef) {
			defer wg.Done()
			_, err := url.ParseRequestURI(releaseRef.URL)
			if err != nil {
				// this is a local release that we need to build/load
				// TODO: support this
				allErrs = multierror.Append(allErrs, fmt.Errorf("Dev release %s is not supported as manifest references", releaseRef.Name))
				return
			}
			// this is a final release that we need to download
			manifestDir := filepath.Dir(m.manifestFilePath)
			finalReleasesWorkDir := filepath.Join(manifestDir, ".final_releases")
			finalReleaseTarballPath := filepath.Join(
				finalReleasesWorkDir,
				fmt.Sprintf("%s-%s-%s.tgz", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))
			finalReleaseUnpackedPath := filepath.Join(
				finalReleasesWorkDir,
				fmt.Sprintf("%s-%s-%s", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))

			if _, err := os.Stat(filepath.Join(finalReleaseUnpackedPath, "release.MF")); err != nil && os.IsNotExist(err) {
				err = os.MkdirAll(finalReleaseUnpackedPath, 0700)
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
					return
				}

				// Show download progress
				bar := progress.AddBar(
					100,
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(
						decor.Name(releaseRef.Name, decor.WCSyncSpaceR),
						decor.Percentage(decor.WCSyncWidth),
					))
				lastPercentage := 0

				// download the release in a directory next to the role manifest
				err = util.DownloadFile(finalReleaseTarballPath, releaseRef.URL, func(percentage int) {
					bar.IncrBy(percentage - lastPercentage)
					lastPercentage = percentage
				})
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
					return
				}
				defer func() {
					os.Remove(finalReleaseTarballPath)
				}()

				// unpack
				err = archiver.TarGz.Open(finalReleaseTarballPath, finalReleaseUnpackedPath)
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
					return
				}
			}
		}(releaseRef)
	}

	wg.Wait()

	// Now that all releases have been downloaded and unpacked,
	// add them to the collection
	for _, releaseRef := range m.Releases {
		manifestDir := filepath.Dir(m.manifestFilePath)
		finalReleasesWorkDir := filepath.Join(manifestDir, ".final_releases")
		finalReleaseUnpackedPath := filepath.Join(
			finalReleasesWorkDir,
			fmt.Sprintf("%s-%s-%s", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))

		// create a release object and add it to the collection
		release, err := NewFinalRelease(finalReleaseUnpackedPath)

		if err != nil {
			allErrs = multierror.Append(allErrs, err)
		}
		releases = append(releases, release)
	}

	return releases, allErrs
}

// resolveRoleManifest takes a role manifest as loaded from disk, and validates
// it to ensure it has no errors, and that the various ancillary structures are
// correctly populated.
func (m *RoleManifest) resolveRoleManifest(grapher util.ModelGrapher) error {
	allErrs := validation.ErrorList{}

	// If template keys are not strings, we need to stop early to avoid panics
	allErrs = append(allErrs, validateTemplateKeysAndValues(m)...)
	if len(allErrs) != 0 {
		return fmt.Errorf(allErrs.Errors())
	}

	mappedReleases, err := m.mappedReleases()
	if err != nil {
		return err
	}

	if grapher != nil {
		for _, release := range m.LoadedReleases {
			grapher.GraphNode("release/"+release.Name, map[string]string{"label": "release/" + release.Name})
		}
	}

	// See also 'GetVariablesForRole' (mustache.go), and LoadRoleManifest (caller, this file)
	declaredConfigs := MakeMapOfVariables(m)

	if m.Configuration.Authorization.Accounts == nil {
		m.Configuration.Authorization.Accounts = make(map[string]AuthAccount)
	}

	if m.Configuration.Authorization.RoleUse == nil {
		m.Configuration.Authorization.RoleUse = make(map[string]int)
	}

	for _, instanceGroup := range m.InstanceGroups {
		// Don't allow any instance groups that are not of the "bosh" or "bosh-task" type
		// Default type is considered to be "bosh".
		// The kept instance groups are validated.
		switch instanceGroup.Type {
		case "":
			instanceGroup.Type = RoleTypeBosh
		case RoleTypeBosh, RoleTypeBoshTask, RoleTypeColocatedContainer:
			// Nothing to do.
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].type", instanceGroup.Name),
				instanceGroup.Type, "Expected one of bosh, bosh-task, or colocated-container"))
		}

		allErrs = append(allErrs, instanceGroup.calculateRoleRun()...)
		allErrs = append(allErrs, validateRoleTags(instanceGroup)...)
		allErrs = append(allErrs, validateRoleRun(instanceGroup, m, declaredConfigs)...)

		// Count how many instance groups use a particular
		// service account. And its roles.

		if instanceGroup.Run != nil {
			account := m.Configuration.Authorization.Accounts[instanceGroup.Run.ServiceAccount]
			account.NumGroups++
			m.Configuration.Authorization.Accounts[instanceGroup.Run.ServiceAccount] = account

			for _, roleName := range account.Roles {
				role := m.Configuration.Authorization.RoleUse[roleName]
				role++
				m.Configuration.Authorization.RoleUse[roleName] = role
			}
		}
	}

	if len(allErrs) != 0 {
		return fmt.Errorf(allErrs.Errors())
	}

	for _, instanceGroup := range m.InstanceGroups {
		instanceGroup.roleManifest = m

		errorList := instanceGroup.Validate(mappedReleases)
		if len(errorList) != 0 {
			allErrs = append(allErrs, errorList...)
		}

		if grapher != nil {
			for _, jobReference := range instanceGroup.JobReferences {
				grapher.GraphNode(jobReference.Job.Fingerprint, map[string]string{"label": "job/" + jobReference.Job.Name})
			}
		}
	}

	// Skip further validation if we fail to resolve any jobs
	// This lets us assume valid jobs in the validation routines
	if len(allErrs) == 0 {
		allErrs = append(allErrs, m.resolveLinks()...)
		allErrs = append(allErrs, validateVariableType(m.Variables)...)
		allErrs = append(allErrs, validateVariableSorting(m.Variables)...)
		allErrs = append(allErrs, validateVariablePreviousNames(m.Variables)...)
		allErrs = append(allErrs, validateVariableUsage(m)...)
		allErrs = append(allErrs, validateTemplateUsage(m, declaredConfigs)...)
		allErrs = append(allErrs, validateServiceAccounts(m)...)
		allErrs = append(allErrs, validateUnusedColocatedContainerRoles(m)...)
		allErrs = append(allErrs, validateColocatedContainerPortCollisions(m)...)
		allErrs = append(allErrs, validateColocatedContainerVolumeShares(m)...)
		allErrs = append(allErrs, validateVariableDescriptions(m)...)
		allErrs = append(allErrs, validateSortedTemplates(m)...)
		allErrs = append(allErrs, validateScripts(m)...)
	}

	if len(allErrs) != 0 {
		return fmt.Errorf(allErrs.Errors())
	}

	return m.resolvePodSecurityPolicies()
}

// resolvePodSecurityPolicies moves the PSP information found in
// RoleManifest.InstanceGroup.JobReferences[].ContainerProperties.BoshContainerization.PodSecurityPolicy to
// RoleManifest.Configuration.Authorization.Accounts[].PodSecurityPolicy
// As service accounts can reference only one PSP the operation makes
// clones of the base SA as needed. Note that the clones reference the same
// roles as the base, and that the roles are not cloned.
func (m *RoleManifest) resolvePodSecurityPolicies() error {
	for _, instanceGroup := range m.InstanceGroups {
		// Note: validateRoleRun ensured non-nil of instanceGroup.Run

		pspName := groupPodSecurityPolicy(instanceGroup)
		accountName := instanceGroup.Run.ServiceAccount
		account, ok := m.Configuration.Authorization.Accounts[accountName]

		if account.PodSecurityPolicy == "" {
			// The account has no PSP information at all.
			// Have it use the PSP of this group
			if !ok {
				m.Configuration.Authorization.Accounts = make(map[string]AuthAccount)
			}
			account.PodSecurityPolicy = pspName
			m.Configuration.Authorization.Accounts[accountName] = account
			continue
		}

		if account.PodSecurityPolicy == pspName {
			// The account's PSP matches the group-requested PSP.
			// There is nothing to do.
			continue
		}

		// The group references a service account which
		// references a different PSP than the group
		// expects. To fix this we:
		// 1. Clone the account
		// 2. Set the clone's PSP to the group's PSP
		// 3. Add the clone to the map, under a new name.
		// 4. Change the group to reference the clone
		//
		// However: The clone may already exist. In that case
		// only step 4 is needed.

		newAccountName := fmt.Sprintf("%s-%s", accountName, pspName)

		if _, ok := m.Configuration.Authorization.Accounts[newAccountName]; !ok {
			newAccount := account
			newAccount.PodSecurityPolicy = pspName

			m.Configuration.Authorization.Accounts[newAccountName] = newAccount
		}

		instanceGroup.Run.ServiceAccount = newAccountName
	}

	return nil
}

func (m *RoleManifest) mappedReleases() (releaseByName, error) {
	mappedReleases := releaseByName{}

	for _, release := range m.LoadedReleases {
		_, ok := mappedReleases[release.Name]

		if ok {
			return mappedReleases, fmt.Errorf("Error - release %s has been loaded more than once", release.Name)
		}

		mappedReleases[release.Name] = release
	}
	return mappedReleases, nil

}

// LookupInstanceGroup will find the given instance group in the role manifest
func (m *RoleManifest) LookupInstanceGroup(name string) *InstanceGroup {
	for _, instanceGroup := range m.InstanceGroups {
		if instanceGroup.Name == name {
			return instanceGroup
		}
	}
	return nil
}

// resolveLinks examines the BOSH links specified in the job specs and maps
// them to the correct role / job that can be looked up at runtime
func (m *RoleManifest) resolveLinks() validation.ErrorList {
	errors := make(validation.ErrorList, 0)

	// Build mappings of providers by name, and by type.  Note that the names
	// involved here are the aliases, where appropriate.
	providersByName := make(map[string]jobProvidesInfo)
	providersByType := make(map[string][]jobProvidesInfo)
	for _, instanceGroup := range m.InstanceGroups {
		for _, jobReference := range instanceGroup.JobReferences {
			var availableProviders []string
			for availableName, availableProvider := range jobReference.Job.AvailableProviders {
				availableProviders = append(availableProviders, availableName)
				if availableProvider.Type != "" {
					providersByType[availableProvider.Type] = append(providersByType[availableProvider.Type], jobProvidesInfo{
						jobLinkInfo: jobLinkInfo{
							Name:     availableProvider.Name,
							Type:     availableProvider.Type,
							RoleName: instanceGroup.Name,
							JobName:  jobReference.Name,
						},
						Properties: availableProvider.Properties,
					})
				}
			}
			for name, provider := range jobReference.ExportedProviders {
				info, ok := jobReference.Job.AvailableProviders[name]
				if !ok {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf("instance_groups[%s].jobs[%s].provides[%s]", instanceGroup.Name, jobReference.Name, name),
						fmt.Sprintf("Provider not found; available providers: %v", availableProviders)))
					continue
				}
				if provider.Alias != "" {
					name = provider.Alias
				}
				providersByName[name] = jobProvidesInfo{
					jobLinkInfo: jobLinkInfo{
						Name:     info.Name,
						Type:     info.Type,
						RoleName: instanceGroup.Name,
						JobName:  jobReference.Name,
					},
					Properties: info.Properties,
				}
			}
		}
	}

	// Resolve the consumers
	for _, instanceGroup := range m.InstanceGroups {
		for _, jobReference := range instanceGroup.JobReferences {
			expectedConsumers := make([]jobConsumesInfo, len(jobReference.Job.DesiredConsumers))
			copy(expectedConsumers, jobReference.Job.DesiredConsumers)
			// Deal with any explicitly marked consumers in the role manifest
			for consumerName, consumerInfo := range jobReference.ResolvedConsumers {
				consumerAlias := consumerName
				if consumerInfo.Alias != "" {
					consumerAlias = consumerInfo.Alias
				}
				if consumerAlias == "" {
					// There was a consumer with an explicitly empty name
					errors = append(errors, validation.Invalid(
						fmt.Sprintf(`instance_group[%s].job[%s]`, instanceGroup.Name, jobReference.Name),
						"name",
						fmt.Sprintf("consumer has no name")))
					continue
				}
				provider, ok := providersByName[consumerAlias]
				if !ok {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf(`instance_group[%s].job[%s].consumes[%s]`, instanceGroup.Name, jobReference.Name, consumerName),
						fmt.Sprintf(`consumer %s not found`, consumerAlias)))
					continue
				}
				jobReference.ResolvedConsumers[consumerName] = jobConsumesInfo{
					jobLinkInfo: provider.jobLinkInfo,
				}

				for i := range expectedConsumers {
					if expectedConsumers[i].Name == consumerName {
						expectedConsumers = append(expectedConsumers[:i], expectedConsumers[i+1:]...)
						break
					}
				}
			}
			// Handle any consumers not overridden in the role manifest
			for _, consumerInfo := range expectedConsumers {
				// Consumers don't _have_ to be listed; they can be automatically
				// matched to a published name, or to the only provider of the
				// same type in the whole deployment
				var provider jobProvidesInfo
				var ok bool
				if consumerInfo.Name != "" {
					provider, ok = providersByName[consumerInfo.Name]
				}
				if !ok && len(providersByType[consumerInfo.Type]) == 1 {
					provider = providersByType[consumerInfo.Type][0]
					ok = true
				}
				if ok {
					name := consumerInfo.Name
					if name == "" {
						name = provider.Name
					}
					info := jobReference.ResolvedConsumers[name]
					info.Name = provider.Name
					info.Type = provider.Type
					info.RoleName = provider.RoleName
					info.JobName = provider.JobName
					jobReference.ResolvedConsumers[name] = info
				} else if !consumerInfo.Optional {
					errors = append(errors, validation.Required(
						fmt.Sprintf(`instance_group[%s].job[%s].consumes[%s]`, instanceGroup.Name, jobReference.Name, consumerInfo.Name),
						fmt.Sprintf(`failed to resolve provider %s (type %s)`, consumerInfo.Name, consumerInfo.Type)))
				}
			}
		}
	}

	return errors
}

// SelectInstanceGroups will find only the given instance groups in the role manifest
func (m *RoleManifest) SelectInstanceGroups(roleNames []string) (InstanceGroups, error) {
	if len(roleNames) == 0 {
		// No names specified, assume all instance groups
		return m.InstanceGroups, nil
	}

	var results InstanceGroups
	var missingRoles []string

	for _, roleName := range roleNames {
		if instanceGroup := m.LookupInstanceGroup(roleName); instanceGroup != nil {
			results = append(results, instanceGroup)
		} else {
			missingRoles = append(missingRoles, roleName)
		}
	}
	if len(missingRoles) > 0 {
		return nil, fmt.Errorf("Some instance groups are unknown: %v", missingRoles)
	}

	return results, nil
}

// normalizeFlightStage reports instance groups with a bad flightstage, and
// fixes all instance groups without a flight stage to use the default
// ('flight').
func normalizeFlightStage(instanceGroup InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// Normalize flight stage
	switch instanceGroup.Run.FlightStage {
	case "":
		instanceGroup.Run.FlightStage = FlightStageFlight
	case FlightStagePreFlight:
	case FlightStageFlight:
	case FlightStagePostFlight:
	case FlightStageManual:
	default:
		allErrs = append(allErrs, validation.Invalid(
			fmt.Sprintf("instance_groups[%s].run.flight-stage", instanceGroup.Name),
			instanceGroup.Run.FlightStage,
			"Expected one of flight, manual, post-flight, or pre-flight"))
	}

	return allErrs
}

func getTemplate(propertyDefs yaml.MapSlice, property string) (interface{}, bool) {
	for _, item := range propertyDefs {
		if item.Key.(string) == property {
			return item.Value, true
		}
	}

	return "", false
}

// groupPodSecurityPolicy determines the name of the pod security policy
// governing the specified instance group.
func groupPodSecurityPolicy(instanceGroup *InstanceGroup) string {
	result := LeastPodSecurityPolicy()

	// Note: validateRoleRun ensured non-nil of job.ContainerProperties.BoshContainerization.PodSecurityPolicy

	for _, job := range instanceGroup.JobReferences {
		result = MergePodSecurityPolicies(result,
			job.ContainerProperties.BoshContainerization.PodSecurityPolicy)
	}

	return result
}
