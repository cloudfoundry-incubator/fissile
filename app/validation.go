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
	errors := make(chan *validation.Error)
	validator, err := newValidator(f, errors)
	if err != nil {
		return validation.ErrorList{err}
	}
	go validator.validate()
	allErrs := validation.ErrorList{}
	for err := range errors {
		allErrs = append(allErrs, err)
	}
	return allErrs
}

type validator struct {
	errOut           chan<- *validation.Error
	f                *Fissile
	propertyDefaults model.PropertyDefaults
	lightOpinions    map[string]string
	darkOpinions     map[string]string
	variableUsage    map[string]int
}

func newValidator(f *Fissile, errOut chan<- *validation.Error) (*validator, *validation.Error) {
	opinions, err := model.NewOpinions(f.Options.LightOpinions, f.Options.DarkOpinions)
	if err != nil {
		return nil, validation.GeneralError("Light are dark opinions could not be read", err)
	}

	return &validator{
		errOut:           errOut,
		f:                f,
		propertyDefaults: f.collectPropertyDefaults(),
		lightOpinions:    model.FlattenOpinions(opinions.Light, false),
		darkOpinions:     model.FlattenOpinions(opinions.Dark, false),
		variableUsage:    make(map[string]int),
	}, nil
}

func (v *validator) validate() {
	defer close(v.errOut)

	v.checkForSortedProperties(
		"configuration.templates",
		v.f.Manifest.Configuration.RawTemplates)

	for propertyName, templateDef := range v.f.Manifest.Configuration.Templates {
		if templateDef.IsGlobal {
			v.checkForUndefinedProperty(
				"configuration.templates",
				propertyName,
				v.propertyDefaults)
		}
	}

	// All light opinions must exists in a bosh release
	for lightProperty := range v.lightOpinions {
		v.checkForUndefinedProperty(
			"light opinion",
			lightProperty,
			v.propertyDefaults)
	}

	// All dark opinions must exists in a bosh release
	// This test is not necessary, as all dark opinions must be overridden by
	// some template, and all templates must come from a used job

	// All dark opinions must be configured as templates
	v.checkForUntemplatedDarkOpinions()

	// No dark opinions must have defaults in light opinions
	v.checkForDarkInTheLight()

	// No duplicates must exist between role manifest and light
	// opinions
	v.checkForDuplicatesBetweenManifestAndLight()

	for _, k := range model.MakeMapOfVariables(v.f.Manifest) {
		v.variableUsage[k.Name] = 0
		if k.CVOptions.Internal {
			// We always count internal variables as used
			v.variableUsage[k.Name]++
		}
	}

	for _, instanceGroup := range v.f.Manifest.InstanceGroups {
		label := fmt.Sprintf("instance_groups[%s].configuration.templates", instanceGroup.Name)
		// Collect the names of properties used in this instance group
		propertyDefaults := instanceGroup.CollectPropertyDefaults()
		for propertyName, templateDef := range instanceGroup.Configuration.Templates {
			if !templateDef.IsGlobal {
				v.checkForUndefinedProperty(label, propertyName, propertyDefaults)
				v.checkForUndefinedVariable(label, propertyName, templateDef.Value)
			}
		}
		v.checkForSortedProperties(
			label,
			instanceGroup.Configuration.RawTemplates)
	}

	// All light opinions should differ from their defaults in the
	// BOSH releases
	v.checkLightDefaults()

	v.checkTemplateInvalidExpansion()
	v.checkNonConstantTemplates()
	v.checkForSortedVariables(v.f.Manifest.Variables)
	for propertyName, templateDef := range v.f.Manifest.Configuration.Templates {
		if templateDef.IsGlobal {
			v.checkForUndefinedVariable("configuration.templates", propertyName, templateDef.Value)
		}
	}
	for variableName, variableUsageCount := range v.variableUsage {
		if variableUsageCount == 0 {
			v.errOut <- validation.NotFound(
				"variables",
				fmt.Sprintf("No templates using '%s'", variableName))
		}
	}
}

// checkForSortedProperties checks that the given ordered YAML map slice have
// all of its keys in order.
func (v *validator) checkForSortedProperties(label string, propertyOrder yaml.MapSlice) {
	var previous string
	for index, property := range propertyOrder {
		key := property.Key.(string)
		if index > 0 {
			if key < previous {
				v.errOut <- validation.Forbidden(
					fmt.Sprintf("%s[%s]", label, previous),
					fmt.Sprintf("Template key does not sort before '%s'", key))
			}
		}
		previous = key
	}
}

// checkForUndefinedProperty checks that a given property is known
func (v *validator) checkForUndefinedProperty(label, propertyName string, knownProperties model.PropertyDefaults) {

	// Ignore specials (without the "properties." prefix)
	if !strings.HasPrefix(propertyName, "properties.") {
		return
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
			return
		}

		v.errOut <- validation.NotFound(
			fmt.Sprintf("%s[%s]", label, propertyName),
			"In any used BOSH job")
	}
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

func (v *validator) checkForSortedVariables(variables model.Variables) {
	previousName := ""
	for _, cv := range variables {
		if cv.Name < previousName {
			v.errOut <- validation.Invalid("variables",
				previousName,
				fmt.Sprintf("Does not sort before '%s'", cv.Name))
		} else if cv.Name == previousName {
			v.errOut <- validation.Invalid("variables",
				previousName, "Appears more than once")
		}
		previousName = cv.Name
	}
}

// checkForUndefinedVariables checks that all configuration templates are
// defined in the variables section
func (v *validator) checkForUndefinedVariable(label, propertyName, templateValue string) {
	varsInTemplate, err := model.ParseTemplate(templateValue)
	if err != nil {
		// Ignore bad template, cannot have sensible
		// variable references
		return
	}
	for _, variable := range varsInTemplate {
		if _, ok := v.variableUsage[variable]; !ok {
			v.errOut <- validation.NotFound(
				fmt.Sprintf("%s[%s]", label, propertyName),
				fmt.Sprintf("No declaration of variable '%s'", variable))
		}
		// Unconditionally increment the usage counter; for variables that
		// are undeclared, this means we won't emit the same warning twice.
		v.variableUsage[variable]++
	}
}

// checkForUntemplatedDarkOpinions reports all dark opinions which are
// not configured as templates in the manifest.
func (v *validator) checkForUntemplatedDarkOpinions() {
	for property := range v.darkOpinions {
		if _, ok := v.f.Manifest.Configuration.Templates[property]; !ok {
			v.errOut <- validation.NotFound(property, "Dark opinion is missing template in role-manifest")
		}
	}

}

// checkForDarkInTheLight reports all dark opinions which have
// defaults in light opinions, which is forbidden
func (v *validator) checkForDarkInTheLight() {
	for property := range v.darkOpinions {
		if _, ok := v.lightOpinions[property]; ok {
			v.errOut <- validation.Forbidden(property, "Dark opinion found in light opinions")
		}
	}
}

// checkTemplateInvalidExpansion reports all templates with syntax errors
func (v *validator) checkTemplateInvalidExpansion() {
	// seenGlobalProperties keeps track of which global properties we've already
	// seen (and checked for duplicates in opinions).
	seenGlobalProperties := make(map[string]struct{})

	for _, instanceGroup := range v.f.Manifest.InstanceGroups {
		groupPrefix := fmt.Sprintf("instance_groups[%s].configuration.templates", instanceGroup.Name)
		for property, template := range instanceGroup.Configuration.Templates {
			prefix := groupPrefix
			if template.IsGlobal {
				if _, ok := seenGlobalProperties[property]; ok {
					continue
				}
				seenGlobalProperties[property] = struct{}{}
				prefix = "configuration.templates"
			}
			if _, err := model.ParseTemplate(template.Value); err != nil {
				v.errOut <- validation.Invalid(
					fmt.Sprintf("%s[%s]", prefix, property),
					template.Value,
					fmt.Sprintf("Template expansion error: %v", err))
			}
		}
	}
}

// checkForDuplicatesBetweenManifestAndLight reports all duplicates
// between role manifest and light opinions, i.e. properties defined
// in both.
func (v *validator) checkForDuplicatesBetweenManifestAndLight() {

	// seenGlobalProperties keeps track of which global properties we've already
	// seen (and checked for duplicates in opinions).
	seenGlobalProperties := make(map[string]struct{})

	for _, instanceGroup := range v.f.Manifest.InstanceGroups {
		groupPrefix := fmt.Sprintf("instance-groups[%s].configuration.templates", instanceGroup.Name)

		for property, template := range instanceGroup.Configuration.Templates {
			prefix := groupPrefix
			if template.IsGlobal {
				if _, ok := seenGlobalProperties[property]; ok {
					continue
				}
				seenGlobalProperties[property] = struct{}{}
				prefix = "configuration.templates"
			}
			if lightValue, ok := v.lightOpinions[property]; ok {
				if lightValue == template.Value {
					// The role manifest _could_ override the opinion, if the
					// opinion has literal values that needs to be run through
					// mustache.
					varsInTemplate, err := model.ParseTemplate(template.Value)
					if err != nil || len(varsInTemplate) == 0 {
						v.errOut <- validation.Forbidden(fmt.Sprintf("%s[%s]", prefix, property),
							"Role-manifest duplicates opinion, remove from manifest")
					}
				} else if template.IsGlobal {
					v.errOut <- validation.Forbidden(fmt.Sprintf("%s[%s]", prefix, property),
						"Role-manifest overrides opinion, remove opinion")
				}
			}
		}
	}
}

// checkLightDefaults reports all light opinions whose value is
// identical to their default in the BOSH releases
func (v *validator) checkLightDefaults() {
	for property, opinion := range v.lightOpinions {
		// Ignore specials (without the "properties." prefix)
		if !strings.HasPrefix(property, "properties.") {
			continue
		}

		// Ignore unknown/undefined property
		pInfo, ok := v.propertyDefaults[strings.TrimPrefix(property, "properties.")]
		if !ok {
			continue
		}

		// Ignore properties with ambiguous defaults.
		if len(pInfo.Defaults) > 1 {
			continue
		}

		if _, ok := pInfo.Defaults[opinion]; ok {
			v.errOut <- validation.Forbidden(property,
				fmt.Sprintf("Light opinion matches default of '%v'", opinion))
		}
	}
}

// checkNonConstantTemplates checks that all templates at the global level use
// some interprolation; constant values should be in opinions instead.
func (v *validator) checkNonConstantTemplates() {
	for key, template := range v.f.Manifest.Configuration.Templates {
		if !template.IsGlobal {
			// Non-global templates are allowed to be constant
			continue
		}
		varsInTemplate, err := model.ParseTemplate(template.Value)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}
		if len(varsInTemplate) == 0 {
			v.errOut <- validation.Forbidden(
				fmt.Sprintf("configuration.templates[%s]", key),
				"Templates used as constants are not allowed")
		}
	}
}
