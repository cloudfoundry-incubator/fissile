package app

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/validation"

	yaml "gopkg.in/yaml.v2"
)

// Validate applies a series of checks to the role
// manifest and opinions, testing for consistency against each other
// and the loaded bosh releases. The result is a (possibly empty)
// array of any issues found.
func (f *Fissile) Validate() validation.ErrorList {
	allErrs := validation.ErrorList{}

	opinions, err := model.NewOpinions(f.Options.LightOpinions, f.Options.DarkOpinions)
	if err != nil {
		return append(allErrs, validation.GeneralError("Light are dark opinions could not be read", err))
	}

	boshPropertyDefaultsAndJobs := f.collectPropertyDefaults()
	darkOpinions := model.FlattenOpinions(opinions.Dark, false)
	lightOpinions := model.FlattenOpinions(opinions.Light, false)

	allErrs = append(allErrs, checkForSortedProperties(
		"configuration.templates",
		f.Manifest.Configuration.RawTemplates)...)

	allErrs = append(allErrs, checkForUndefinedProperties(
		"configuration.templates",
		true,
		f.Manifest.Configuration.Templates,
		boshPropertyDefaultsAndJobs)...)

	// All light opinions must exists in a bosh release
	usedLightProperties := map[string]model.ConfigurationTemplate{}
	for lightProperty := range lightOpinions {
		usedLightProperties[lightProperty] = model.ConfigurationTemplate{
			IsGlobal: true,
		}
	}
	allErrs = append(allErrs, checkForUndefinedProperties(
		"light opinion",
		true,
		usedLightProperties, boshPropertyDefaultsAndJobs)...)

	// All dark opinions must exists in a bosh release
	// This test is not necessary, as all dark opinions must be overridden by
	// some template, and all templates must come from a used job

	// All dark opinions must be configured as templates
	allErrs = append(allErrs, checkForUntemplatedDarkOpinions(darkOpinions,
		f.Manifest.Configuration.Templates)...)

	// No dark opinions must have defaults in light opinions
	allErrs = append(allErrs, checkForDarkInTheLight(darkOpinions, lightOpinions)...)

	// No duplicates must exist between role manifest and light
	// opinions
	allErrs = append(allErrs, checkForDuplicatesBetweenManifestAndLight(lightOpinions, f.Manifest)...)

	variableUsage := map[string]int{}
	for _, k := range model.MakeMapOfVariables(f.Manifest) {
		variableUsage[k.Name] = 0
		if k.CVOptions.Internal {
			// We always count internal variables as used
			variableUsage[k.Name]++
		}
	}

	allPropertyDefaults := model.PropertyDefaults{}
	for _, instanceGroup := range f.Manifest.InstanceGroups {
		// Collect the names of properties used in this instance group
		propertyDefaults := instanceGroup.CollectPropertyDefaults()
		allErrs = append(allErrs, checkForUndefinedProperties(
			fmt.Sprintf("instance_groups[%s].configuration.templates", instanceGroup.Name),
			false,
			instanceGroup.Configuration.Templates,
			propertyDefaults)...)
		allErrs = append(allErrs, checkForUndefinedVariables(
			fmt.Sprintf("instance_groups[%s].configuration.templates", instanceGroup.Name),
			instanceGroup.Configuration.Templates,
			variableUsage,
			false)...)

		// Collect all property defaults across all instance groups for a global
		// check later.
		for propertyName, defaults := range propertyDefaults {
			if _, ok := allPropertyDefaults[propertyName]; !ok {
				allPropertyDefaults[propertyName] = model.NewPropertyInfo()
			}
			for v := range defaults.Defaults {
				allPropertyDefaults[propertyName].Defaults[v] = append(allPropertyDefaults[propertyName].Defaults[v], defaults.Defaults[v]...)
			}
			if defaults.MaybeHash {
				allPropertyDefaults[propertyName].MaybeHash = true
			}
		}

		allErrs = append(allErrs, checkForSortedProperties(
			fmt.Sprintf("instance_groups[%s].configuration.templates", instanceGroup.Name),
			instanceGroup.Configuration.RawTemplates)...)
	}

	// All light opinions should differ from their defaults in the
	// BOSH releases
	allErrs = append(allErrs, checkLightDefaults(lightOpinions,
		allPropertyDefaults)...)

	allErrs = append(allErrs, checkNonConstantTemplates(f.Manifest.Configuration.RawTemplates)...)
	allErrs = append(allErrs, checkForSortedVariables(f.Manifest.Variables)...)
	allErrs = append(allErrs, checkForUndefinedVariables(
		"configuration.templates",
		f.Manifest.Configuration.Templates,
		variableUsage,
		true)...)
	for variableName, variableUsageCount := range variableUsage {
		if variableUsageCount == 0 {
			allErrs = append(allErrs, validation.NotFound(
				"variables",
				fmt.Sprintf("No templates using '%s'", variableName)))
		}
	}

	return allErrs
}

// checkForSortedProperties checks that the given ordered YAML map slice have
// all of its keys in order.
func checkForSortedProperties(label string, propertyOrder yaml.MapSlice) validation.ErrorList {
	var previous string
	allErrs := validation.ErrorList{}

	for index, property := range propertyOrder {
		key := property.Key.(string)
		if index > 0 {
			if key < previous {
				allErrs = append(allErrs, validation.Forbidden(
					fmt.Sprintf("%s[%s]", label, previous),
					fmt.Sprintf("Template key does not sort before '%s'", key)))
			}
		}
		previous = key
	}

	return allErrs
}

// checkForUndefinedProperties checks that all properties are known
func checkForUndefinedProperties(label string, global bool, templates map[string]model.ConfigurationTemplate, knownProperties model.PropertyDefaults) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for propertyName, templateDef := range templates {
		// Ignore global templates when checking instance-group specific ones,
		// and vice versa.
		if templateDef.IsGlobal != global {
			continue
		}
		// Ignore specials (without the "properties." prefix)
		if !strings.HasPrefix(propertyName, "properties.") {
			continue
		}

		p := strings.TrimPrefix(propertyName, "properties.")
		if _, ok := knownProperties[p]; !ok {
			// The property as is was not found. This is
			// not necessarily an error. The "property"
			// may actually part of the value for a
			// structured (hash) property. To determine
			// this we walk the chain of parents to see if
			// any of them exist, and report an error only
			// if none of them do.

			if checkParentsOfUndefined(p, knownProperties) {
				continue
			}

			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("%s[%s]", label, propertyName),
				"In any used BOSH job"))
		}
	}

	return allErrs
}

// checkParentsOfUndefined walks the chain of parents for `p` from the
// bottom up and checks if any of them exist. The elements of the
// chain are separated by dots.
func checkParentsOfUndefined(p string, defaults model.PropertyDefaults) bool {
	at := strings.LastIndex(p, ".")

	for at >= 0 {
		// While there is a dot in the property name we have a
		// parent to check the existence of

		parent := p[0:at]

		if pInfo, ok := defaults[parent]; ok {
			// We have a possible parent. Look if that
			// candidate may be a hash. If not our
			// property cannot be valid.

			if pInfo.MaybeHash {
				return true
			}

			return false
		}

		p = parent
		at = strings.LastIndex(p, ".")
	}

	return false
}

func checkForSortedVariables(variables model.Variables) validation.ErrorList {
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

// checkForUndefinedVariables checks that all configuration templates are
// defined in the variables section
func checkForUndefinedVariables(label string, templates map[string]model.ConfigurationTemplate, variableUsage map[string]int, isGlobal bool) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for propertyName, template := range templates {
		if template.IsGlobal != isGlobal {
			continue
		}
		varsInTemplate, err := model.ParseTemplate(template.Value)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}
		for _, variable := range varsInTemplate {
			if _, ok := variableUsage[variable]; !ok {
				allErrs = append(allErrs, validation.NotFound(
					fmt.Sprintf("%s[%s]", label, propertyName),
					fmt.Sprintf("No declaration of variable '%s'", variable)))
			}
			// Unconditionally increment the usage counter; for variables that
			// are undeclared, this means we won't emit the same warning twice.
			variableUsage[variable]++
		}
	}

	return allErrs
}

// checkForUntemplatedDarkOpinions reports all dark opinions which are
// not configured as templates in the manifest.
func checkForUntemplatedDarkOpinions(dark map[string]string, templates map[string]model.ConfigurationTemplate) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for property := range dark {
		if _, ok := templates[property]; ok {
			continue
		}
		allErrs = append(allErrs, validation.NotFound(
			property, "Dark opinion is missing template in role-manifest"))
	}

	return allErrs
}

// checkForDarkInTheLight reports all dark opinions which have
// defaults in light opinions, which is forbidden
func checkForDarkInTheLight(dark map[string]string, light map[string]string) validation.ErrorList {
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

// checkForDuplicatesBetweenManifestAndLight reports all duplicates
// between role manifest and light opinions, i.e. properties defined
// in both.
func checkForDuplicatesBetweenManifestAndLight(light map[string]string, roleManifest *model.RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	check := make(map[string]struct{})

	for _, instanceGroup := range roleManifest.InstanceGroups {
		prefix := fmt.Sprintf("instance-groups[%s].configuration.templates", instanceGroup.Name)

		for property, template := range instanceGroup.Configuration.Templates {
			if template.IsGlobal {
				if _, ok := check[property]; ok {
					// Skip over duplicates of the global properties in the
					// per-instance-group data, we already checked them in a
					// different role
					continue
				}
				check[property] = struct{}{}
				allErrs = append(allErrs, checkForDuplicateProperty("configuration.templates", property, template.Value, light, true)...)
			} else {
				allErrs = append(allErrs, checkForDuplicateProperty(prefix, property, template.Value, light, false)...)
			}
		}
	}

	return allErrs
}

// checkForDuplicateProperty performs the check for a property (of the
// manifest) duplicated in the light opinions.
func checkForDuplicateProperty(prefix, property, value string, light map[string]string, conflicts bool) validation.ErrorList {
	allErrs := validation.ErrorList{}

	lightValue, ok := light[property]
	if !ok {
		return allErrs
	}

	if lightValue == value {
		return append(allErrs, validation.Forbidden(fmt.Sprintf("%s[%s]", prefix, property),
			"Role-manifest duplicates opinion, remove from manifest"))
	}

	if conflicts {
		return append(allErrs, validation.Forbidden(fmt.Sprintf("%s[%s]", prefix, property),
			"Role-manifest overrides opinion, remove opinion"))
	}

	return allErrs
}

// checkLightDefaults reports all light opinions whose value is
// identical to their default in the BOSH releases
func checkLightDefaults(light map[string]string, pd model.PropertyDefaults) validation.ErrorList {

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
		pInfo, ok := pd[p]
		if !ok {
			continue
		}

		// Ignore properties with ambiguous defaults.
		if len(pInfo.Defaults) > 1 {
			continue
		}

		if _, ok := pInfo.Defaults[opinion]; ok {
			allErrs = append(allErrs, validation.Forbidden(property,
				fmt.Sprintf("Light opinion matches default of '%v'",
					opinion)))
		}
	}

	return allErrs
}

// checkNonConstantTemplates checks that all templates at the global level use
// some interprolation; constant values should be in opinions instead.
func checkNonConstantTemplates(templates yaml.MapSlice) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, templateDef := range templates {
		key, ok := templateDef.Key.(string)
		if !ok {
			// Invalid property; this should be reported earlier
			continue
		}
		template := templateDef.Value.(string)
		varsInTemplate, err := model.ParseTemplate(template)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}
		if len(varsInTemplate) == 0 {
			allErrs = append(allErrs, validation.Forbidden(
				fmt.Sprintf("configuration.templates[%s]", key),
				"Templates used as constants are not allowed"))
		}
	}

	return allErrs
}
