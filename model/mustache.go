package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hpcloud/fissile/mustache"
)

// MakeMapOfVariables converts the sequence of configuration variables
// into a map we can manipulate more directly by name.
func MakeMapOfVariables(rolesManifest *RoleManifest) CVMap {
	configsDictionary := CVMap{}

	for _, config := range rolesManifest.Configuration.Variables {
		configsDictionary[config.Name] = config
	}

	return configsDictionary
}

// GetVariablesForRole returns all the environment variables required for
// calculating all the templates for the role
func (r *Role) GetVariablesForRole() (ConfigurationVariableSlice, error) {

	configsDictionary := MakeMapOfVariables(r.rolesManifest)

	configs := CVMap{}

	for _, job := range r.Jobs {
		for _, property := range job.Properties {
			propertyName := fmt.Sprintf("properties.%s", property.Name)

			for templatePropName, template := range r.Configuration.Templates {
				switch true {
				case templatePropName == propertyName:
				case strings.HasPrefix(templatePropName, propertyName+"."):
				default:
					// Not a matching property
					continue
				}
				varsInTemplate, err := parseTemplate(template)
				if err != nil {
					return nil, err
				}

				for _, envVar := range varsInTemplate {
					if confVar, ok := configsDictionary[envVar]; ok {
						configs[confVar.Name] = confVar
					}
				}
			}
		}
	}

	result := make(ConfigurationVariableSlice, 0, len(configs))

	for _, value := range configs {
		result = append(result, value)
	}

	sort.Sort(result)

	return result, nil
}

func parseTemplate(template string) ([]string, error) {

	parsed, err := mustache.ParseString(fmt.Sprintf("{{=(( ))=}}%s", template))

	if err != nil {
		return nil, err
	}

	return parsed.GetTemplateVariables(), nil
}
