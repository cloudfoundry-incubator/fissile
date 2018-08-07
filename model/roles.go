package model

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"sort"
	"strings"

	"github.com/SUSE/fissile/util"
	"github.com/SUSE/fissile/validation"

	"gopkg.in/yaml.v2"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	InstanceGroups InstanceGroups `yaml:"instance_groups"`
	Configuration  *Configuration `yaml:"configuration"`
	Variables      Variables

	manifestFilePath string
}

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, releases []*Release, grapher util.ModelGrapher) (*RoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	roleManifest := RoleManifest{}
	roleManifest.manifestFilePath = manifestFilePath
	if err := yaml.Unmarshal(manifestContents, &roleManifest); err != nil {
		return nil, err
	}
	if roleManifest.Configuration == nil {
		roleManifest.Configuration = &Configuration{}
	}
	if roleManifest.Configuration.Templates == nil {
		roleManifest.Configuration.Templates = map[string]string{}
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

	err = roleManifest.resolveRoleManifest(releases, grapher)
	if err != nil {
		return nil, err
	}
	return &roleManifest, nil
}

// resolveRoleManifest takes a role manifest as loaded from disk, and validates
// it to ensure it has no errors, and that the various ancillary structures are
// correctly populated.
func (m *RoleManifest) resolveRoleManifest(releases []*Release, grapher util.ModelGrapher) error {
	mappedReleases := map[string]*Release{}

	for _, release := range releases {
		_, ok := mappedReleases[release.Name]

		if ok {
			return fmt.Errorf("Error - release %s has been loaded more than once", release.Name)
		}

		mappedReleases[release.Name] = release
		if grapher != nil {
			grapher.GraphNode("release/"+release.Name, map[string]string{"label": "release/" + release.Name})
		}
	}

	// See also 'GetVariablesForRole' (mustache.go).
	declaredConfigs := MakeMapOfVariables(m)

	allErrs := validation.ErrorList{}

	for i := len(m.InstanceGroups) - 1; i >= 0; i-- {
		instanceGroup := m.InstanceGroups[i]

		// Remove all instance groups that are not of the "bosh" or "bosh-task" type
		// Default type is considered to be "bosh".
		// The kept instance groups are validated.
		switch instanceGroup.Type {
		case "":
			instanceGroup.Type = RoleTypeBosh
		case RoleTypeBosh, RoleTypeBoshTask, RoleTypeColocatedContainer:
			// Nothing to do.
		case RoleTypeDocker:
			m.InstanceGroups = append(m.InstanceGroups[:i], m.InstanceGroups[i+1:]...)
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].type", instanceGroup.Name),
				instanceGroup.Type, "Expected one of bosh, bosh-task, docker, or colocated-container"))
		}

		allErrs = append(allErrs, validateRoleTags(instanceGroup)...)
		allErrs = append(allErrs, validateRoleRun(instanceGroup, m, declaredConfigs)...)
	}

	for _, instanceGroup := range m.InstanceGroups {
		instanceGroup.roleManifest = m

		if instanceGroup.Run != nil && instanceGroup.Run.ActivePassiveProbe != "" {
			if !instanceGroup.HasTag(RoleTagActivePassive) {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].run.active-passive-probe", instanceGroup.Name),
					instanceGroup.Run.ActivePassiveProbe,
					"Active/passive probes are only valid on instance groups with active-passive tag"))
			}
		}

		for _, jobReference := range instanceGroup.JobReferences {
			release, ok := mappedReleases[jobReference.ReleaseName]

			if !ok {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].jobs[%s]", instanceGroup.Name, jobReference.Name),
					jobReference.ReleaseName,
					"Referenced release is not loaded"))
				continue
			}

			job, err := release.LookupJob(jobReference.Name)
			if err != nil {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].jobs[%s]", instanceGroup.Name, jobReference.Name),
					jobReference.ReleaseName, err.Error()))
				continue
			}

			jobReference.Job = job
			if grapher != nil {
				_ = grapher.GraphNode(job.Fingerprint, map[string]string{"label": "job/" + job.Name})
			}

			if jobReference.ResolvedConsumers == nil {
				// No explicitly specified consumers
				jobReference.ResolvedConsumers = make(map[string]jobConsumesInfo)
			}

			for name, info := range jobReference.ResolvedConsumers {
				info.Name = name
				jobReference.ResolvedConsumers[name] = info
			}
		}

		instanceGroup.calculateRoleConfigurationTemplates()

		// Validate that specified colocated containers are configured and of the
		// correct type
		for idx, roleName := range instanceGroup.ColocatedContainers {
			if lookupRole := m.LookupInstanceGroup(roleName); lookupRole == nil {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", instanceGroup.Name, idx),
					roleName,
					"There is no such instance group defined"))

			} else if lookupRole.Type != RoleTypeColocatedContainer {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", instanceGroup.Name, idx),
					roleName,
					"The instance group is not of required type colocated-container"))
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
		allErrs = append(allErrs, validateTemplateUsage(m)...)
		allErrs = append(allErrs, validateNonTemplates(m)...)
		allErrs = append(allErrs, validateServiceAccounts(m)...)
		allErrs = append(allErrs, validateUnusedColocatedContainerRoles(m)...)
		allErrs = append(allErrs, validateColocatedContainerPortCollisions(m)...)
		allErrs = append(allErrs, validateColocatedContainerVolumeShares(m)...)
	}

	if len(allErrs) != 0 {
		return fmt.Errorf(allErrs.Errors())
	}

	return nil
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

// validateVariableType checks that only legal values are used for
// the type field of variables, and resolves missing information to
// defaults. It reports all variables which are badly typed.
func validateVariableType(variables Variables) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, cv := range variables {
		switch cv.CVOptions.Type {
		case "":
			cv.CVOptions.Type = CVTypeUser
		case CVTypeUser:
		case CVTypeEnv:
			if cv.CVOptions.Internal {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("configuration.variables[%s].type", cv.Name),
					cv.CVOptions.Type, `type conflicts with flag "internal"`))
			}
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("configuration.variables[%s].type", cv.Name),
				cv.CVOptions.Type, "Expected one of user, or environment"))
		}
	}

	return allErrs
}

// validateVariableSorting tests whether the parameters are properly sorted or not.
// It reports all variables which are out of order.
func validateVariableSorting(variables Variables) validation.ErrorList {
	allErrs := validation.ErrorList{}

	previousName := ""
	for _, cv := range variables {
		if cv.Name < previousName {
			allErrs = append(allErrs, validation.Invalid("configuration.variables",
				previousName,
				fmt.Sprintf("Does not sort before '%s'", cv.Name)))
		} else if cv.Name == previousName {
			allErrs = append(allErrs, validation.Invalid("configuration.variables",
				previousName, "Appears more than once"))
		}
		previousName = cv.Name
	}

	return allErrs
}

// validateVariablePreviousNames tests whether PreviousNames of a variable are used either
// by as a Name or a PreviousName of another variable.
func validateVariablePreviousNames(variables Variables) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, cvOuter := range variables {
		for _, previousOuter := range cvOuter.CVOptions.PreviousNames {
			for _, cvInner := range variables {
				if previousOuter == cvInner.Name {
					allErrs = append(allErrs, validation.Invalid("configuration.variables",
						cvOuter.Name,
						fmt.Sprintf("Previous name '%s' also exist as a new variable", cvInner.Name)))
				}
				for _, previousInner := range cvInner.CVOptions.PreviousNames {
					if cvOuter.Name != cvInner.Name && previousOuter == previousInner {
						allErrs = append(allErrs, validation.Invalid("configuration.variables",
							cvOuter.Name,
							fmt.Sprintf("Previous name '%s' also claimed by '%s'", previousOuter, cvInner.Name)))
					}
				}
			}
		}
	}

	return allErrs
}

// validateVariableUsage tests whether all parameters are used in a
// template or not.  It reports all variables which are not used by at
// least one template.  An exception are the variables marked with
// `internal: true`. These are not reported.  Use this to declare
// variables used only in the various scripts and not in templates.
// See also the notes on type `ConfigurationVariable` in this file.
func validateVariableUsage(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// See also 'GetVariablesForRole' (mustache.go).

	unusedConfigs := MakeMapOfVariables(roleManifest)
	if len(unusedConfigs) == 0 {
		return allErrs
	}

	// Iterate over all roles, jobs, templates, extract the used
	// variables. Remove each found from the set of unused
	// configs.

	for _, role := range roleManifest.InstanceGroups {
		for _, jobReference := range role.JobReferences {
			for _, property := range jobReference.Properties {
				propertyName := fmt.Sprintf("properties.%s", property.Name)

				if template, ok := role.Configuration.Templates[propertyName]; ok {
					varsInTemplate, err := parseTemplate(template)
					if err != nil {
						// Ignore bad template, cannot have sensible
						// variable references
						continue
					}
					for _, envVar := range varsInTemplate {
						if _, ok := unusedConfigs[envVar]; ok {
							delete(unusedConfigs, envVar)
						}
						if len(unusedConfigs) == 0 {
							// Everything got used, stop now.
							return allErrs
						}
					}
				}
			}
		}
	}

	// Iterate over the global templates, extract the used
	// variables. Remove each found from the set of unused
	// configs.

	// Note, we have to ignore bad templates (no sensible variable
	// references) and continue to check everything else.

	for _, template := range roleManifest.Configuration.Templates {
		varsInTemplate, err := parseTemplate(template)
		if err != nil {
			continue
		}
		for _, envVar := range varsInTemplate {
			if _, ok := unusedConfigs[envVar]; ok {
				delete(unusedConfigs, envVar)
			}
			if len(unusedConfigs) == 0 {
				// Everything got used, stop now.
				return allErrs
			}
		}
	}

	// We have only the unused variables left in the set. Report
	// those which are not internal.

	for cv, cvar := range unusedConfigs {
		if cvar.CVOptions.Internal {
			continue
		}

		allErrs = append(allErrs, validation.NotFound("configuration.variables",
			fmt.Sprintf("No templates using '%s'", cv)))
	}

	return allErrs
}

// validateTemplateUsage tests whether all templates use only declared variables or not.
// It reports all undeclared variables.
func validateTemplateUsage(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// See also 'GetVariablesForRole' (mustache.go), and LoadRoleManifest (caller, this file)
	declaredConfigs := MakeMapOfVariables(roleManifest)

	// Iterate over all instance groups, jobs, templates, extract the used
	// variables. Report all without a declaration.

	for _, instanceGroup := range roleManifest.InstanceGroups {

		// Note, we cannot use GetVariablesForRole here
		// because it will abort on bad templates. Here we
		// have to ignore them (no sensible variable
		// references) and continue to check everything else.

		for _, jobReference := range instanceGroup.JobReferences {
			for _, property := range jobReference.Properties {
				propertyName := fmt.Sprintf("properties.%s", property.Name)

				if template, ok := instanceGroup.Configuration.Templates[propertyName]; ok {
					varsInTemplate, err := parseTemplate(template)
					if err != nil {
						continue
					}
					for _, envVar := range varsInTemplate {
						if _, ok := declaredConfigs[envVar]; ok {
							continue
						}

						allErrs = append(allErrs, validation.NotFound("configuration.variables",
							fmt.Sprintf("No declaration of '%s'", envVar)))

						// Add a placeholder so that this variable is not reported again.
						// One report is good enough.
						declaredConfigs[envVar] = nil
					}
				}
			}
		}
	}

	// Iterate over the global templates, extract the used
	// variables. Report all without a declaration.

	for _, template := range roleManifest.Configuration.Templates {
		varsInTemplate, err := parseTemplate(template)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}
		for _, envVar := range varsInTemplate {
			if _, ok := declaredConfigs[envVar]; ok {
				continue
			}

			allErrs = append(allErrs, validation.NotFound("configuration.templates",
				fmt.Sprintf("No variable declaration of '%s'", envVar)))

			// Add a placeholder so that this variable is
			// not reported again.  One report is good
			// enough.
			declaredConfigs[envVar] = nil
		}
	}

	return allErrs
}

// validateRoleRun tests whether required fields in the RoleRun are
// set. Note, some of the fields have type-dependent checks. Some
// issues are fixed silently.
func validateRoleRun(instanceGroup *InstanceGroup, roleManifest *RoleManifest, declared CVMap) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run == nil {
		return append(allErrs, validation.Required(
			fmt.Sprintf("instance_groups[%s].run", instanceGroup.Name), ""))
	}

	if instanceGroup.Run.Scaling != nil && instanceGroup.Run.Scaling.HA == 0 {
		instanceGroup.Run.Scaling.HA = instanceGroup.Run.Scaling.Min
	}

	allErrs = append(allErrs, normalizeFlightStage(instanceGroup)...)
	allErrs = append(allErrs, validateHealthCheck(instanceGroup)...)
	allErrs = append(allErrs, validateRoleMemory(instanceGroup)...)
	allErrs = append(allErrs, validateRoleCPU(instanceGroup)...)

	for _, exposedPort := range instanceGroup.Run.ExposedPorts {
		allErrs = append(allErrs, ValidateExposedPorts(instanceGroup.Name, exposedPort)...)
	}

	if instanceGroup.Run.ServiceAccount != "" {
		accountName := instanceGroup.Run.ServiceAccount
		if _, ok := roleManifest.Configuration.Authorization.Accounts[accountName]; !ok {
			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("instance_groups[%s].run.service-account", instanceGroup.Name), accountName))
		}
	}

	// Backwards compat: convert separate volume lists to the centralized one
	for _, persistentVolume := range instanceGroup.Run.PersistentVolumes {
		persistentVolume.Type = VolumeTypePersistent
		instanceGroup.Run.Volumes = append(instanceGroup.Run.Volumes, persistentVolume)
	}
	for _, sharedVolume := range instanceGroup.Run.SharedVolumes {
		sharedVolume.Type = VolumeTypeShared
		instanceGroup.Run.Volumes = append(instanceGroup.Run.Volumes, sharedVolume)
	}
	instanceGroup.Run.PersistentVolumes = nil
	instanceGroup.Run.SharedVolumes = nil
	for _, volume := range instanceGroup.Run.Volumes {
		switch volume.Type {
		case VolumeTypePersistent:
		case VolumeTypeShared:
		case VolumeTypeHost:
		case VolumeTypeNone:
		case VolumeTypeEmptyDir:
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.volumes[%s]", instanceGroup.Name, volume.Tag),
				volume.Type,
				fmt.Sprintf("Invalid volume type '%s'", volume.Type)))
		}
	}

	// Normalize capabilities to upper case, if any.
	var capabilities []string
	for _, cap := range instanceGroup.Run.Capabilities {
		capabilities = append(capabilities, strings.ToUpper(cap))
	}
	instanceGroup.Run.Capabilities = capabilities

	if len(instanceGroup.Run.Environment) == 0 {
		return allErrs
	}

	if instanceGroup.Type == RoleTypeDocker {
		// The environment variables used by docker roles must
		// all be declared, report those which are not.

		for _, envVar := range instanceGroup.Run.Environment {
			if _, ok := declared[envVar]; ok {
				continue
			}

			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("instance_groups[%s].run.env", instanceGroup.Name),
				fmt.Sprintf("No variable declaration of '%s'", envVar)))
		}
	} else {
		// Bosh instance groups must not provide environment variables.

		allErrs = append(allErrs, validation.Forbidden(
			fmt.Sprintf("instance_groups[%s].run.env", instanceGroup.Name),
			"Non-docker instance group declares bogus parameters"))
	}

	return allErrs
}

// ValidateExposedPorts validates exposed port ranges. It also translates the legacy
// format of port ranges ("2000-2010") into the FirstPort and Count values.
func ValidateExposedPorts(name string, exposedPorts *RoleRunExposedPort) validation.ErrorList {
	allErrs := validation.ErrorList{}

	fieldName := fmt.Sprintf("instance_groups[%s].run.exposed-ports[%s]", name, exposedPorts.Name)

	// Validate Name
	if exposedPorts.Name == "" {
		allErrs = append(allErrs, validation.Required(fieldName+".name", ""))
	} else if regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$").FindString(exposedPorts.Name) == "" {
		allErrs = append(allErrs, validation.Invalid(fieldName+".name", exposedPorts.Name,
			"port names must be lowercase words separated by hyphens"))
	}
	if len(exposedPorts.Name) > 15 {
		allErrs = append(allErrs, validation.Invalid(fieldName+".name", exposedPorts.Name,
			"port name must be no more than 15 characters"))
	} else if len(exposedPorts.Name) > 9 && exposedPorts.CountIsConfigurable {
		// need to be able to append "-12345" and still be 15 chars or less
		allErrs = append(allErrs, validation.Invalid(fieldName+".name", exposedPorts.Name,
			"user configurable port name must be no more than 9 characters"))
	}

	// Validate Protocol
	allErrs = append(allErrs, validation.ValidateProtocol(exposedPorts.Protocol, fieldName+".protocol")...)

	// Validate Internal
	firstPort, lastPort, errs := validation.ValidatePortRange(exposedPorts.Internal, fieldName+".internal")
	allErrs = append(allErrs, errs...)
	exposedPorts.InternalPort = firstPort

	if exposedPorts.Count == 0 {
		exposedPorts.Count = lastPort + 1 - firstPort
	} else if lastPort+1-firstPort != exposedPorts.Count {
		allErrs = append(allErrs, validation.Invalid(fieldName+".count", exposedPorts.Count,
			fmt.Sprintf("count doesn't match port range %s", exposedPorts.Internal)))
	}

	// Validate External
	if exposedPorts.External == "" {
		exposedPorts.ExternalPort = exposedPorts.InternalPort
	} else {
		firstPort, lastPort, errs := validation.ValidatePortRange(exposedPorts.External, fieldName+".external")
		allErrs = append(allErrs, errs...)
		exposedPorts.ExternalPort = firstPort

		if lastPort+1-firstPort != exposedPorts.Count {
			allErrs = append(allErrs, validation.Invalid(fieldName+".external", exposedPorts.External,
				"internal and external ranges must have same number of ports"))
		}
	}

	if exposedPorts.Max == 0 {
		exposedPorts.Max = exposedPorts.Count
	}

	// Validate default port count; actual count will be validated at deploy time
	if exposedPorts.Count > exposedPorts.Max {
		allErrs = append(allErrs, validation.Invalid(fieldName+".count", exposedPorts.Count,
			fmt.Sprintf("default number of ports %d is larger than max allowed %d",
				exposedPorts.Count, exposedPorts.Max)))
	}

	// Clear out legacy fields to make sure they aren't still be used elsewhere in the code
	exposedPorts.Internal = ""
	exposedPorts.External = ""

	return allErrs
}

// validateRoleMemory validates memory requests and limits, and
// converts the old key (`memory`, run.MemRequest), to the new
// form. Afterward only run.Memory is valid.
func validateRoleMemory(instanceGroup *InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run.Memory == nil {
		if instanceGroup.Run.MemRequest != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.MemRequest,
				fmt.Sprintf("instance_groups[%s].run.memory", instanceGroup.Name))...)
		}
		instanceGroup.Run.Memory = &RoleRunMemory{Request: instanceGroup.Run.MemRequest}
		return allErrs
	}

	// assert: role.Run.Memory != nil

	if instanceGroup.Run.Memory.Request == nil {
		if instanceGroup.Run.MemRequest != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.MemRequest,
				fmt.Sprintf("instance_groups[%s].run.memory", instanceGroup.Name))...)
		}
		instanceGroup.Run.Memory.Request = instanceGroup.Run.MemRequest
	} else {
		allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.Memory.Request,
			fmt.Sprintf("instance_groups[%s].run.mem.request", instanceGroup.Name))...)
	}

	if instanceGroup.Run.Memory.Limit != nil {
		allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.Memory.Limit,
			fmt.Sprintf("instance_groups[%s].run.mem.limit", instanceGroup.Name))...)
	}

	return allErrs
}

// validateRoleCPU validates cpu requests and limits, and converts the
// old key (`virtual-cpus`, run.VirtualCPUs), to the new
// form. Afterward only run.CPU is valid.
func validateRoleCPU(instanceGroup *InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run.CPU == nil {
		if instanceGroup.Run.VirtualCPUs != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.VirtualCPUs,
				fmt.Sprintf("instance_groups[%s].run.virtual-cpus", instanceGroup.Name))...)
		}
		instanceGroup.Run.CPU = &RoleRunCPU{Request: instanceGroup.Run.VirtualCPUs}
		return allErrs
	}

	// assert: role.Run.CPU != nil

	if instanceGroup.Run.CPU.Request == nil {
		if instanceGroup.Run.VirtualCPUs != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.VirtualCPUs,
				fmt.Sprintf("instance_groups[%s].run.virtual-cpus", instanceGroup.Name))...)
		}
		instanceGroup.Run.CPU.Request = instanceGroup.Run.VirtualCPUs
	} else {
		allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.CPU.Request,
			fmt.Sprintf("instance_groups[%s].run.cpu.request", instanceGroup.Name))...)
	}

	if instanceGroup.Run.CPU.Limit != nil {
		allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.CPU.Limit,
			fmt.Sprintf("instance_groups[%s].run.cpu.limit", instanceGroup.Name))...)
	}

	return allErrs
}

// validateHealthCheck reports a instance group with conflicting health checks
// in its probes
func validateHealthCheck(instanceGroup *InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run.HealthCheck == nil {
		// No health checks, nothing to validate
		return allErrs
	}

	// Ensure that we don't have conflicting health checks
	if instanceGroup.Run.HealthCheck.Readiness != nil {
		allErrs = append(allErrs,
			validateHealthProbe(instanceGroup, "readiness",
				instanceGroup.Run.HealthCheck.Readiness)...)
	}
	if instanceGroup.Run.HealthCheck.Liveness != nil {
		allErrs = append(allErrs,
			validateHealthProbe(instanceGroup, "liveness",
				instanceGroup.Run.HealthCheck.Liveness)...)
	}

	return allErrs
}

// validateHealthProbe reports a instance group with conflicting health checks
// in the specified probe.
func validateHealthProbe(instanceGroup *InstanceGroup, probeName string, probe *HealthProbe) validation.ErrorList {
	allErrs := validation.ErrorList{}

	checks := make([]string, 0, 3)
	if probe.URL != "" {
		checks = append(checks, "url")
	}
	if len(probe.Command) > 0 {
		checks = append(checks, "command")
	}
	if probe.Port != 0 {
		checks = append(checks, "port")
	}
	if len(checks) > 1 {
		allErrs = append(allErrs, validation.Invalid(
			fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
			checks, "Expected at most one of url, command, or port"))
	}
	switch instanceGroup.Type {

	case RoleTypeBosh:
		if len(checks) == 0 {
			allErrs = append(allErrs, validation.Required(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s.command", instanceGroup.Name, probeName),
				"Health check requires a command"))
		} else if checks[0] != "command" {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
				checks, "Only command health checks are supported for BOSH instance groups"))
		} else if probeName != "readiness" && len(probe.Command) > 1 {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s.command", instanceGroup.Name, probeName),
				probe.Command, fmt.Sprintf("%s check can only have one command", probeName)))
		}

	case RoleTypeBoshTask:
		if len(checks) > 0 {
			allErrs = append(allErrs, validation.Forbidden(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
				"bosh-task instance groups cannot have health checks"))
		}

	case RoleTypeDocker:
		if len(probe.Command) > 1 {
			allErrs = append(allErrs, validation.Forbidden(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
				"docker instance groups do not support multiple commands"))
		}

	default:
		// We should have caught the invalid role type when loading the role manifest
		panic("Unexpected role type " + string(instanceGroup.Type) + " in instance group " + instanceGroup.Name)
	}

	return allErrs
}

// normalizeFlightStage reports instance groups with a bad flightstage, and
// fixes all instance groups without a flight stage to use the default
// ('flight').
func normalizeFlightStage(instanceGroup *InstanceGroup) validation.ErrorList {
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

// validateNonTemplates tests whether the global templates are
// constant or not. It reports the contant templates as errors (They
// should be opinions).
func validateNonTemplates(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// Iterate over the global templates, extract the used
	// variables. Report all templates not using any variable.

	for property, template := range roleManifest.Configuration.Templates {
		varsInTemplate, err := parseTemplate(template)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}

		if len(varsInTemplate) == 0 {
			allErrs = append(allErrs, validation.Invalid("configuration.templates",
				template,
				fmt.Sprintf("Using '%s' as a constant", property)))
		}
	}

	return allErrs
}

func validateServiceAccounts(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}
	for accountName, accountInfo := range roleManifest.Configuration.Authorization.Accounts {
		for _, roleName := range accountInfo.Roles {
			if _, ok := roleManifest.Configuration.Authorization.Roles[roleName]; !ok {
				allErrs = append(allErrs, validation.NotFound(
					fmt.Sprintf("configuration.auth.accounts[%s].roles", accountName),
					roleName))
			}
		}
	}
	return allErrs
}

func validateUnusedColocatedContainerRoles(RoleManifest *RoleManifest) validation.ErrorList {
	counterMap := map[string]int{}
	for _, instanceGroup := range RoleManifest.InstanceGroups {
		// Initialise any instance group of type colocated container in the counter map
		if instanceGroup.Type == RoleTypeColocatedContainer {
			if _, ok := counterMap[instanceGroup.Name]; !ok {
				counterMap[instanceGroup.Name] = 0
			}
		}

		// Increase counter of configured colocated container names
		for _, roleName := range instanceGroup.ColocatedContainers {
			if _, ok := counterMap[roleName]; !ok {
				counterMap[roleName] = 0
			}

			counterMap[roleName]++
		}
	}

	allErrs := validation.ErrorList{}
	for roleName, usageCount := range counterMap {
		if usageCount == 0 {
			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("instance_group[%s]", roleName),
				"instance group is of type colocated container, but is not used by any other instance group as such"))
		}
	}

	return allErrs
}

func validateColocatedContainerPortCollisions(RoleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, instanceGroup := range RoleManifest.InstanceGroups {
		if len(instanceGroup.ColocatedContainers) > 0 {
			lookupMap := map[string][]string{}

			// Iterate over this instance group and all colocated container instance groups and store the
			// names of the groups for each protocol/port (there should be no list with
			// more than one entry)
			for _, toBeChecked := range append([]*InstanceGroup{instanceGroup}, instanceGroup.GetColocatedRoles()...) {
				for _, exposedPort := range toBeChecked.Run.ExposedPorts {
					for i := 0; i < exposedPort.Count; i++ {
						protocolPortTuple := fmt.Sprintf("%s/%d", exposedPort.Protocol, exposedPort.ExternalPort+i)
						if _, ok := lookupMap[protocolPortTuple]; !ok {
							lookupMap[protocolPortTuple] = []string{}
						}

						lookupMap[protocolPortTuple] = append(lookupMap[protocolPortTuple], toBeChecked.Name)
					}
				}
			}

			// Get a sorted list of the keys (protocol/port)
			protocolPortTuples := []string{}
			for protocolPortTuple := range lookupMap {
				protocolPortTuples = append(protocolPortTuples, protocolPortTuple)
			}
			sort.Strings(protocolPortTuples)

			// Now check if we have port collisions
			for _, protocolPortTuple := range protocolPortTuples {
				names := lookupMap[protocolPortTuple]

				if len(names) > 1 {
					allErrs = append(allErrs, validation.Invalid(
						fmt.Sprintf("instance_group[%s]", instanceGroup.Name),
						protocolPortTuple,
						fmt.Sprintf("port collision, the same protocol/port is used by: %s", strings.Join(names, ", "))))
				}
			}
		}
	}

	return allErrs
}

func validateRoleTags(instanceGroup *InstanceGroup) validation.ErrorList {
	var allErrs validation.ErrorList

	acceptableRoleTypes := map[RoleTag][]RoleType{
		RoleTagActivePassive:     []RoleType{RoleTypeBosh},
		RoleTagHeadless:          []RoleType{RoleTypeBosh, RoleTypeDocker},
		RoleTagSequentialStartup: []RoleType{RoleTypeBosh, RoleTypeDocker},
		RoleTagStopOnFailure:     []RoleType{RoleTypeBoshTask},
	}

	for tagNum, tag := range instanceGroup.Tags {
		switch tag {
		case RoleTagStopOnFailure:
		case RoleTagSequentialStartup:
		case RoleTagHeadless:
		case RoleTagActivePassive:
			if instanceGroup.Run == nil || instanceGroup.Run.ActivePassiveProbe == "" {
				allErrs = append(allErrs, validation.Required(
					fmt.Sprintf("instance_groups[%s].run.active-passive-probe", instanceGroup.Name),
					"active-passive instance groups must specify the correct probe"))
			}
			if instanceGroup.HasTag(RoleTagHeadless) {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
					tag,
					"headless instance groups may not be active-passive"))
			}

		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
				string(tag), "Unknown tag"))
			continue
		}

		if _, ok := acceptableRoleTypes[tag]; !ok {
			allErrs = append(allErrs, validation.InternalError(
				fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
				fmt.Errorf("Tag %s has no acceptable role list", tag)))
			continue
		}

		validTypeForTag := false
		for _, roleType := range acceptableRoleTypes[tag] {
			if roleType == instanceGroup.Type {
				validTypeForTag = true
				break
			}
		}
		if !validTypeForTag {
			var roleNames []string
			for _, roleType := range acceptableRoleTypes[tag] {
				roleNames = append(roleNames, string(roleType))
			}
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
				tag,
				fmt.Sprintf("%s tag is only supported in [%s] instance groups, not %s", tag, strings.Join(roleNames, ", "), instanceGroup.Type)))
		}
	}

	return allErrs
}

func validateColocatedContainerVolumeShares(RoleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, instanceGroup := range RoleManifest.InstanceGroups {
		numberOfColocatedContainers := len(instanceGroup.ColocatedContainers)

		if numberOfColocatedContainers > 0 {
			emptyDirVolumesTags := []string{}
			emptyDirVolumesPath := map[string]string{}
			emptyDirVolumesCount := map[string]int{}

			// Compile a map of all emptyDir volumes with tag -> path of the main instance group
			for _, volume := range instanceGroup.Run.Volumes {
				if volume.Type == VolumeTypeEmptyDir {
					emptyDirVolumesTags = append(emptyDirVolumesTags, volume.Tag)
					emptyDirVolumesPath[volume.Tag] = volume.Path
					emptyDirVolumesCount[volume.Tag] = 0
				}
			}

			for _, colocatedRole := range instanceGroup.GetColocatedRoles() {
				for _, volume := range colocatedRole.Run.Volumes {
					if volume.Type == VolumeTypeEmptyDir {
						if _, ok := emptyDirVolumesCount[volume.Tag]; !ok {
							emptyDirVolumesCount[volume.Tag] = 0
						}

						emptyDirVolumesCount[volume.Tag]++

						if path, ok := emptyDirVolumesPath[volume.Tag]; ok {
							if path != volume.Path {
								// Same tag, but different paths
								allErrs = append(allErrs, validation.Invalid(
									fmt.Sprintf("instance_group[%s]", colocatedRole.Name),
									volume.Path,
									fmt.Sprintf("colocated instance group specifies a shared volume with tag %s, which path does not match the path of the main instance group shared volume with the same tag", volume.Tag)))
							}
						}
					}
				}
			}

			// Check the counters
			sort.Strings(emptyDirVolumesTags)
			for _, tag := range emptyDirVolumesTags {
				count := emptyDirVolumesCount[tag]
				if count != numberOfColocatedContainers {
					allErrs = append(allErrs, validation.Required(
						fmt.Sprintf("instance_group[%s]", instanceGroup.Name),
						fmt.Sprintf("container must use shared volumes of the main instance group: %s", tag)))
				}
			}
		}
	}

	return allErrs
}
