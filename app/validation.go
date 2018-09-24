package app

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry-incubator/fissile/model"
	"github.com/cloudfoundry-incubator/fissile/validation"
)

// validateManifestAndOpinions applies a series of checks to the role
// manifest and opinions, testing for consistency against each other
// and the loaded bosh releases. The result is a (possibly empty)
// array of any issues found.
func (f *Fissile) validateManifestAndOpinions(roleManifest *model.RoleManifest, opinions *model.Opinions, defaultsFromEnvFiles map[string]string) validation.ErrorList {
	allErrs := validation.ErrorList{}

	boshPropertyDefaultsAndJobs := f.collectPropertyDefaults()
	darkOpinions := model.FlattenOpinions(opinions.Dark, false)
	lightOpinions := model.FlattenOpinions(opinions.Light, false)
	manifestProperties := collectManifestProperties(roleManifest)

	// All properties must be defined in a BOSH release
	allErrs = append(allErrs, checkForUndefinedBOSHProperties("role-manifest",
		manifestProperties, boshPropertyDefaultsAndJobs)...)

	// All light opinions must exists in a bosh release
	allErrs = append(allErrs, checkForUndefinedBOSHProperties("light opinion",
		lightOpinions, boshPropertyDefaultsAndJobs)...)

	// All dark opinions must exists in a bosh release
	allErrs = append(allErrs, checkForUndefinedBOSHProperties("dark opinion",
		darkOpinions, boshPropertyDefaultsAndJobs)...)

	// All dark opinions must be configured as templates
	allErrs = append(allErrs, checkForUntemplatedDarkOpinions(darkOpinions,
		manifestProperties)...)

	// No dark opinions must have defaults in light opinions
	allErrs = append(allErrs, checkForDarkInTheLight(darkOpinions, lightOpinions)...)

	// No duplicates must exist between role manifest and light
	// opinions
	allErrs = append(allErrs, checkForDuplicatesBetweenManifestAndLight(lightOpinions, roleManifest)...)

	// All vars in env files must exist in the role manifest
	allErrs = append(allErrs, f.checkEnvFileVariables(roleManifest, defaultsFromEnvFiles)...)

	// All light opinions should differ from their defaults in the
	// BOSH releases
	allErrs = append(allErrs, f.checkLightDefaults(lightOpinions,
		boshPropertyDefaultsAndJobs)...)

	return allErrs
}

// Check that the given 'properties' are all defined in a 'bosh'
// release.
func checkForUndefinedBOSHProperties(label string, properties map[string]string, bosh propertyDefaults) validation.ErrorList {
	// All provided properties must be defined in a BOSH release
	allErrs := validation.ErrorList{}

	for property := range properties {
		// Ignore specials (without the "properties." prefix)
		if !strings.HasPrefix(property, "properties.") {
			continue
		}
		p := strings.TrimPrefix(property, "properties.")

		if _, ok := bosh[p]; !ok {
			// The property as is was not found. This is
			// not necessarily an error. The "property"
			// may actually part of the value for a
			// structured (hash) property. To determine
			// this we walk the chain of parents to see if
			// any of them exist, and report an error only
			// if none of them do.

			if checkParentsOfUndefined(p, bosh) {
				continue
			}

			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("%s '%s'", label, p), "In any BOSH release"))
		}
	}

	return allErrs
}

// checkParentsOfUndefined walks the chain of parents for `p` from the
// bottom up and checks if any of them exist. The elements of the
// chain are separated by dots.
func checkParentsOfUndefined(p string, bosh propertyDefaults) bool {
	at := strings.LastIndex(p, ".")

	for at >= 0 {
		// While there is a dot in the property name we have a
		// parent to check the existence of

		tail := p[at:]
		parent := strings.TrimSuffix(p, tail)

		if pInfo, ok := bosh[parent]; ok {
			// We have a possible parent. Look if that
			// candidate may be a hash. If not our
			// property cannot be valid.

			if pInfo.maybeHash {
				return true
			}

			return false
		}

		p = parent
		at = strings.LastIndex(p, ".")
	}

	return false
}

// collectManifestProperties returns a map merging the global and
// per-instance-group properties/templates into a single structure.
func collectManifestProperties(roleManifest *model.RoleManifest) map[string]string {
	properties := make(map[string]string)

	// Per-instance-group properties
	for _, instanceGroup := range roleManifest.InstanceGroups {
		for _, propertyDef := range instanceGroup.Configuration.Templates {
			property := propertyDef.Key.(string)
			template := propertyDef.Value

			properties[property] = fmt.Sprintf("%v", template)
		}
	}

	// And the global properties
	for _, propertyDef := range roleManifest.Configuration.Templates {
		property := propertyDef.Key.(string)
		template := propertyDef.Value.(string)

		properties[property] = template
	}

	return properties
}

// checkForUntemplatedDarkOpinions reports all dark opinions which are
// not configured as templates in the manifest.
func checkForUntemplatedDarkOpinions(dark map[string]string, properties map[string]string) validation.ErrorList {
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

	// The global properties, ...
	for _, propertyDef := range roleManifest.Configuration.Templates {
		property := propertyDef.Key.(string)
		template := propertyDef.Value.(string)

		allErrs = append(allErrs, checkForDuplicateProperty("configuration.templates", property, template, light, true)...)
		check[property] = struct{}{}
	}

	// ... then the per-instance-group properties
	for _, instanceGroup := range roleManifest.InstanceGroups {
		prefix := fmt.Sprintf("instance-groups[%s].configuration.templates", instanceGroup.Name)

		for _, propertyDef := range instanceGroup.Configuration.Templates {
			property := propertyDef.Key.(string)
			template := propertyDef.Value

			// Skip over duplicates of the global
			// properties in the per-instance-group data, we already
			// checked them, see above.
			if _, ok := check[property]; ok {
				continue
			}
			allErrs = append(allErrs, checkForDuplicateProperty(prefix, property, fmt.Sprintf("%v", template), light, false)...)
		}
	}

	return allErrs
}

// checkForDuplicateProperty performs the check for a property (of the
// manifest) duplicated in the light opinions.
func checkForDuplicateProperty(prefix, property, value string, light map[string]string, conflicts bool) validation.ErrorList {
	allErrs := validation.ErrorList{}

	lightvalue, ok := light[property]
	if !ok {
		return allErrs
	}

	if lightvalue == value {
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
func (f *Fissile) checkLightDefaults(light map[string]string, pd propertyDefaults) validation.ErrorList {

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

		// Ignore properties with ambigous defaults.
		if len(pInfo.defaults) > 1 {
			continue
		}

		// len(pInfo.defaults) == 1 --> This loop will run only once
		// Is there a better (more direct?) way to get the
		// single key, i.e. default from the map ?
		for thedefault := range pInfo.defaults {
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

// All vars in env files must exist in the role manifest
func (f *Fissile) checkEnvFileVariables(roleManifest *model.RoleManifest, defaults map[string]string) validation.ErrorList {
	allErrs := validation.ErrorList{}

	exists := func(key string) bool {
		for _, variable := range roleManifest.Variables {
			if variable.Name == key {
				return true
			}
		}

		return false
	}

	for key := range defaults {
		if !exists(key) {
			allErrs = append(allErrs, validation.NotFound(key,
				"Variable from env file not defined in the manifest."))
		}
	}

	return allErrs
}
