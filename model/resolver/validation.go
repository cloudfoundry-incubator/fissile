package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/validation"
)

// Validate implements several checks for the instance group and its job references. It's run after the
// instance groups are filtered and i.e. Run has been calculated.
// It adds the releases Job spec to the instance groups JobReferences
func validateInstanceGroup(roleManifest *model.RoleManifest, g *model.InstanceGroup, releaseResolver model.ReleaseResolver) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if g.Run.ActivePassiveProbe != "" {
		if !g.HasTag(model.RoleTagActivePassive) {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.active-passive-probe", g.Name),
				g.Run.ActivePassiveProbe,
				"Active/passive probes are only valid on instance groups with active-passive tag"))
		}
	}

	for _, jobReference := range g.JobReferences {
		release, ok := releaseResolver.FindRelease(jobReference.ReleaseName)
		if !ok {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].jobs[%s]", g.Name, jobReference.Name),
				jobReference.ReleaseName,
				"Referenced release is not loaded"))
			continue
		}

		job, err := release.LookupJob(jobReference.Name)
		if err != nil {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].jobs[%s]", g.Name, jobReference.Name),
				jobReference.ReleaseName, err.Error()))
			continue
		}
		jobReference.Job = job

		if jobReference.ResolvedConsumers == nil {
			// No explicitly specified consumers
			jobReference.ResolvedConsumers = make(map[string]model.JobConsumesInfo)
		}

		for name, info := range jobReference.ResolvedConsumers {
			info.Name = name
			jobReference.ResolvedConsumers[name] = info
		}
	}

	g.CalculateRoleConfigurationTemplates()

	// Validate that specified colocated containers are configured and of the
	// correct type
	for idx, roleName := range g.ColocatedContainers() {
		if lookupRole := roleManifest.LookupInstanceGroup(roleName); lookupRole == nil {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", g.Name, idx),
				roleName,
				"There is no such instance group defined"))

		} else if lookupRole.Type != model.RoleTypeColocatedContainer {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", g.Name, idx),
				roleName,
				"The instance group is not of required type colocated-container"))
		}
	}

	return allErrs
}

// validateTemplateKeys tests whether all template keys are strings and that
// global template values are strings
func validateTemplateKeysAndValues(roleManifest *model.RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, instanceGroup := range roleManifest.InstanceGroups {
		if instanceGroup.Configuration == nil {
			continue
		}

		for _, templateDef := range instanceGroup.Configuration.Templates {
			if _, ok := templateDef.Key.(string); !ok {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("template key for instance group %s", instanceGroup.Name),
					templateDef.Key,
					"Template key must be a string"))
			}
		}
	}

	if roleManifest.Configuration == nil {
		return allErrs
	}

	for _, templateDef := range roleManifest.Configuration.Templates {
		if _, ok := templateDef.Key.(string); !ok {
			allErrs = append(allErrs, validation.Invalid(
				"global template key",
				templateDef.Key,
				"Template key must be a string"))
		}

		if _, ok := templateDef.Value.(string); !ok {
			allErrs = append(allErrs, validation.Invalid(
				"global template value",
				templateDef.Key,
				"Template value must be a string"))
		}
	}

	return allErrs
}

func validateRoleTags(instanceGroup *model.InstanceGroup) validation.ErrorList {
	var allErrs validation.ErrorList

	acceptableRoleTypes := map[model.RoleTag][]model.RoleType{
		model.RoleTagActivePassive:     []model.RoleType{model.RoleTypeBosh},
		model.RoleTagSequentialStartup: []model.RoleType{model.RoleTypeBosh},
		model.RoleTagStopOnFailure:     []model.RoleType{model.RoleTypeBoshTask},
		model.RoleTagIstioManaged:      []model.RoleType{model.RoleTypeBosh},
	}

	for tagNum, tag := range instanceGroup.Tags {
		switch tag {
		case model.RoleTagIstioManaged:
		case model.RoleTagStopOnFailure:
		case model.RoleTagSequentialStartup:
		case model.RoleTagActivePassive:
			if instanceGroup.Run == nil || instanceGroup.Run.ActivePassiveProbe == "" {
				allErrs = append(allErrs, validation.Required(
					fmt.Sprintf("instance_groups[%s].run.active-passive-probe", instanceGroup.Name),
					"active-passive instance groups must specify the correct probe"))
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

// validateVariableType checks that only legal values are used for
// the type field of variables, and resolves missing information to
// defaults. It reports all variables which are badly typed.
func validateVariableType(variables model.Variables) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, cv := range variables {
		switch cv.Type {
		case "":
		case "certificate":
		case "password":
		case "ssh":
		case "rsa":
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("variables[%s].type", cv.Name),
				cv.Type, "The rsa type is not yet supported by the secret generator"))
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("variables[%s].type", cv.Name),
				cv.Type, "Expected one of certificate, password, rsa, ssh or empty"))
		}

		switch cv.CVOptions.Type {
		case "":
			cv.CVOptions.Type = model.CVTypeUser
		case model.CVTypeUser:
		case model.CVTypeEnv:
			if cv.CVOptions.Internal {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("variables[%s].options.type", cv.Name),
					cv.CVOptions.Type, `type conflicts with flag "internal"`))
			}
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("variables[%s].options.type", cv.Name),
				cv.CVOptions.Type, "Expected one of user, or environment"))
		}
	}

	return allErrs
}

// validateVariableSorting tests whether the parameters are properly sorted or not.
// It reports all variables which are out of order.
func validateVariableSorting(variables model.Variables) validation.ErrorList {
	allErrs := validation.ErrorList{}

	previousName := ""
	for _, cv := range variables {
		if cv.Name < previousName {
			allErrs = append(allErrs, validation.Invalid("variables",
				previousName,
				fmt.Sprintf("Does not sort before '%s'", cv.Name)))
		} else if cv.Name == previousName {
			allErrs = append(allErrs, validation.Invalid("variables",
				previousName, "Appears more than once"))
		}
		previousName = cv.Name
	}

	return allErrs
}

// validateVariablePreviousNames tests whether PreviousNames of a variable are used either
// by as a Name or a PreviousName of another variable.
func validateVariablePreviousNames(variables model.Variables) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, cvOuter := range variables {
		for _, previousOuter := range cvOuter.CVOptions.PreviousNames {
			for _, cvInner := range variables {
				if previousOuter == cvInner.Name {
					allErrs = append(allErrs, validation.Invalid("variables",
						cvOuter.Name,
						fmt.Sprintf("Previous name '%s' also exist as a new variable", cvInner.Name)))
				}
				for _, previousInner := range cvInner.CVOptions.PreviousNames {
					if cvOuter.Name != cvInner.Name && previousOuter == previousInner {
						allErrs = append(allErrs, validation.Invalid("variables",
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
func validateVariableUsage(roleManifest *model.RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// See also 'GetVariablesForRole' (mustache.go).
	unusedConfigs := model.MakeMapOfVariables(roleManifest)
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

				if template, ok := model.GetTemplate(role.Configuration.Templates, propertyName); ok {
					varsInTemplate, err := model.ParseTemplate(fmt.Sprintf("%v", template))
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
	for _, propertyDef := range roleManifest.Configuration.Templates {
		template := propertyDef.Value.(string)

		varsInTemplate, err := model.ParseTemplate(template)
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

		allErrs = append(allErrs, validation.NotFound("variables",
			fmt.Sprintf("No templates using '%s'", cv)))
	}

	return allErrs
}

// validateTemplateUsage tests whether all templates use only declared variables or not.
// It reports all undeclared variables.
func validateTemplateUsage(roleManifest *model.RoleManifest, declaredConfigs model.CVMap) validation.ErrorList {
	allErrs := validation.ErrorList{}

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

				if template, ok := model.GetTemplate(instanceGroup.Configuration.Templates, propertyName); ok {
					varsInTemplate, err := model.ParseTemplate(fmt.Sprintf("%v", template))
					if err != nil {
						continue
					}
					for _, envVar := range varsInTemplate {
						if _, ok := declaredConfigs[envVar]; ok {
							continue
						}

						allErrs = append(allErrs, validation.NotFound("variables",
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
	for _, templateDef := range roleManifest.Configuration.Templates {
		key := templateDef.Key.(string)
		template := templateDef.Value.(string)
		varsInTemplate, err := model.ParseTemplate(template)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}

		if len(varsInTemplate) == 0 {
			allErrs = append(allErrs, validation.Forbidden(key,
				"Templates used as constants are not allowed"))
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

func validateServiceAccounts(roleManifest *model.RoleManifest) validation.ErrorList {
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

func validateUnusedColocatedContainerRoles(roleManifest *model.RoleManifest) validation.ErrorList {
	counterMap := map[string]int{}
	for _, instanceGroup := range roleManifest.InstanceGroups {
		// Initialise any instance group of type colocated container in the counter map
		if instanceGroup.Type == model.RoleTypeColocatedContainer {
			if _, ok := counterMap[instanceGroup.Name]; !ok {
				counterMap[instanceGroup.Name] = 0
			}
		}

		for _, j := range instanceGroup.JobReferences {

			// Increase counter of configured colocated container names
			for _, roleName := range j.ContainerProperties.BoshContainerization.ColocatedContainers {
				if _, ok := counterMap[roleName]; !ok {
					counterMap[roleName] = 0
				}

				counterMap[roleName]++
			}
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

func validateColocatedContainerPortCollisions(roleManifest *model.RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, instanceGroup := range roleManifest.InstanceGroups {
		if len(instanceGroup.ColocatedContainers()) > 0 {
			lookupMap := map[string][]string{}

			// Iterate over this instance group and all colocated container instance groups and store the
			// names of the groups for each protocol/port (there should be no list with
			// more than one entry)
			for _, toBeChecked := range append(model.InstanceGroups{instanceGroup}, instanceGroup.GetColocatedRoles()...) {
				for _, j := range toBeChecked.JobReferences {
					for _, exposedPort := range j.ContainerProperties.BoshContainerization.Ports {
						for i := 0; i < exposedPort.Count; i++ {
							protocolPortTuple := fmt.Sprintf("%s/%d", exposedPort.Protocol, exposedPort.ExternalPort+i)
							if _, ok := lookupMap[protocolPortTuple]; !ok {
								lookupMap[protocolPortTuple] = []string{}
							}

							lookupMap[protocolPortTuple] = append(lookupMap[protocolPortTuple], toBeChecked.Name)
						}
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

func validateColocatedContainerVolumeShares(roleManifest *model.RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, instanceGroup := range roleManifest.InstanceGroups {
		numberOfColocatedContainers := len(instanceGroup.ColocatedContainers())

		if numberOfColocatedContainers > 0 {
			emptyDirVolumesTags := []string{}
			emptyDirVolumesPath := map[string]string{}
			emptyDirVolumesCount := map[string]int{}

			// Compile a map of all emptyDir volumes with tag -> path of the main instance group
			for _, volume := range instanceGroup.Run.Volumes {
				if volume.Type == model.VolumeTypeEmptyDir {
					emptyDirVolumesTags = append(emptyDirVolumesTags, volume.Tag)
					emptyDirVolumesPath[volume.Tag] = volume.Path
					emptyDirVolumesCount[volume.Tag] = 0
				}
			}

			for _, colocatedRole := range instanceGroup.GetColocatedRoles() {
				for _, volume := range colocatedRole.Run.Volumes {
					if volume.Type == model.VolumeTypeEmptyDir {
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

// validateVariableDescriptions tests whether all variables have descriptions
func validateVariableDescriptions(roleManifest *model.RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, variable := range roleManifest.Variables {
		if variable.CVOptions.Description == "" {
			allErrs = append(allErrs, validation.Required(variable.Name,
				"Description is required"))
		}
	}

	return allErrs
}

// validateSortedTemplates tests that all templates are sorted in alphabetical order
func validateSortedTemplates(roleManifest *model.RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	previousKey := ""

	for _, templateDef := range roleManifest.Configuration.Templates {
		key := templateDef.Key.(string)

		if previousKey != "" && previousKey > key {
			allErrs = append(allErrs, validation.Forbidden(previousKey,
				fmt.Sprintf("Template key does not sort before '%s'", key)))
		}

		previousKey = key
	}

	return allErrs
}

// validateScripts tests that all referenced scripts exist, and that all scripts
// are referenced.
func validateScripts(roleManifest *model.RoleManifest, validationOptions model.RoleManifestValidationOptions) validation.ErrorList {
	allErrs := validation.ErrorList{}
	roleManifestDirName := filepath.Dir(roleManifest.ManifestFilePath)
	scriptsDirName := filepath.Join(roleManifestDirName, "scripts")
	usedScripts := map[string]bool{}
	err := filepath.Walk(scriptsDirName, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir // No need to walk hidden directories
			}
			return nil // Ignore all hidden files
		}
		if info.IsDir() {
			return nil // Ignore directories, but recurse into them
		}

		relpath, err := filepath.Rel(scriptsDirName, path)
		if err != nil {
			return err
		}
		usedScripts["scripts/"+relpath] = false
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return append(allErrs, validation.Invalid(scriptsDirName, err.Error(), "Error listing files in scripts directory"))
	}

	for _, instanceGroup := range roleManifest.InstanceGroups {
		for scriptType, scriptList := range map[string][]string{
			"script":             instanceGroup.Scripts,
			"environment script": instanceGroup.EnvironScripts,
			"post config script": instanceGroup.PostConfigScripts,
		} {
			for _, script := range scriptList {
				if filepath.IsAbs(script) {
					// We allow scripts with absolute paths, as they (likely) come from BOSH packages rather than being
					// provided with the role manifest.
					continue
				}
				if !filepath.HasPrefix(script, "scripts/") {
					allErrs = append(allErrs, validation.Invalid(
						fmt.Sprintf("%s %s", instanceGroup.Name, scriptType),
						script,
						"Script path does not start with scripts/"))
				}
				if !validationOptions.AllowMissingScripts {
					if _, ok := usedScripts[script]; !ok {
						allErrs = append(allErrs, validation.Invalid(
							fmt.Sprintf("%s %s", instanceGroup.Name, scriptType),
							script,
							"script not found"))
					}
				}
				usedScripts[script] = true
			}
		}
	}

	if !validationOptions.AllowMissingScripts {
		for scriptName, scriptUsed := range usedScripts {
			if !scriptUsed {
				allErrs = append(allErrs, validation.Required(scriptName, "Script is not used"))
			}
		}
	}

	return allErrs
}

// validateHealthProbe reports a instance group with conflicting health checks
// in the specified probe.
func validateHealthProbe(instanceGroup model.InstanceGroup, probeName string, probe *model.HealthProbe) validation.ErrorList {
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

	case model.RoleTypeBosh:
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

	case model.RoleTypeBoshTask:
		if len(checks) > 0 {
			allErrs = append(allErrs, validation.Forbidden(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
				"bosh-task instance groups cannot have health checks"))
		}

	default:
		// We should have caught the invalid role type when loading the role manifest
		panic("Unexpected role type " + string(instanceGroup.Type) + " in instance group " + instanceGroup.Name)
	}

	return allErrs
}