package model

import (
	"fmt"

	"github.com/hpcloud/fissile/mustache"
)

// GetVariablesForRole returns all the environment variables required for
// calculating all the templates for the role
func (r *Role) GetVariablesForRole() ([]*ConfigurationVariable, error) {

	configsDictionary := map[string]*ConfigurationVariable{}

	for _, config := range r.rolesManifest.Configuration.Variables {
		configsDictionary[config.Name] = config
	}

	configs := map[string]*ConfigurationVariable{}

	for _, job := range r.Jobs {
		for _, property := range job.Properties {
			propertyName := fmt.Sprintf("properties.%s", property.Name)
			if template, ok := r.rolesManifest.Configuration.Templates[propertyName]; ok {

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

	result := []*ConfigurationVariable{}

	for _, value := range configs {
		result = append(result, value)
	}

	return result, nil
}

func parseTemplate(template string) ([]string, error) {

	parsed, err := mustache.ParseString(fmt.Sprintf("{{=(( ))=}}%s", template))

	if err != nil {
		return nil, err
	}

	return parsed.GetTemplateVariables(), nil
}
