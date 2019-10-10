package model

import (
	"fmt"
	"sort"
	"strings"

	"code.cloudfoundry.org/fissile/mustache"
)

// MakeMapOfVariables converts the sequence of configuration variables
// into a map we can manipulate more directly by name.
func MakeMapOfVariables(roleManifest *RoleManifest) CVMap {
	configsDictionary := CVMap{}

	for _, v := range roleManifest.Variables {
		configsDictionary[v.Name] = v
	}

	for _, config := range builtins() {
		configsDictionary[config.Name] = config
	}

	return configsDictionary
}

// GetVariablesForRole returns all the environment variables required for
// calculating all the templates for the role
func (g *InstanceGroup) GetVariablesForRole() (Variables, error) {

	configsDictionary := MakeMapOfVariables(g.roleManifest)

	configs := CVMap{}

	// First, render all referenced variables of type user.

	for _, jobReference := range g.JobReferences {
		for _, property := range jobReference.Properties {
			propertyName := fmt.Sprintf("properties.%s", property.Name)

			for templatePropName, template := range g.Configuration.Templates {

				switch true {
				case templatePropName == propertyName:
				case strings.HasPrefix(templatePropName, propertyName+"."):
				default:
					// Not a matching property
					continue
				}

				varsInTemplate, err := ParseTemplate(template.Value)
				if err != nil {
					return nil, err
				}

				for _, envVar := range varsInTemplate {
					if confVar, ok := configsDictionary[envVar]; ok {
						if confVar.CVOptions.Type == CVTypeUser {
							configs[confVar.Name] = confVar
						} else if confVar.CVOptions.Type == CVTypeEnv && confVar.CVOptions.Default != "" {
							configs[confVar.Name] = confVar
						}
					}
				}
			}
		}
	}

	// Second, render all user-variables which are marked as
	// internal. The reasoning: Being internal, i.e. used by
	// scripts, but not templates, we cannot know for sure that
	// they are not used, so we err on the side of caution and
	// assume usage, therefore render.

	for _, confVar := range configsDictionary {
		if confVar.CVOptions.Type == CVTypeUser && confVar.CVOptions.Internal {
			configs[confVar.Name] = confVar
		}
	}

	configs["KUBERNETES_CONTAINER_NAME"] = &VariableDefinition{
		Name: "KUBERNETES_CONTAINER_NAME",
		CVOptions: CVOptions{
			Type:    CVTypeEnv,
			Default: g.Name,
		},
	}

	result := make(Variables, 0, len(configs))

	for _, value := range configs {
		result = append(result, value)
	}

	sort.Sort(result)

	return result, nil
}

// ParseTemplate parses a mustache template and returns the template variables
func ParseTemplate(template string) ([]string, error) {

	parsed, err := mustache.ParseString(fmt.Sprintf("{{=(( ))=}}%s", template))

	if err != nil {
		return nil, err
	}

	return parsed.GetTemplateVariables(), nil
}

func builtins() Variables {
	// Fissile provides some configuration variables by itself,
	// see --> scripts/dockerfiles/run.sh, add them to prevent
	// them from being reported as errors.  The code here has to
	// match the list of variables there.

	// Notes:
	// - Type `environment` because they are supplied by the
	//   runtime code, not the user.
	// - Internal, because while a user can reference them in
	//   the templates of a role manifest it does not have to.
	// - As they are added to the data structures after the loaded
	//   RM is validated this does __not__ mean that a user can
	//   specify variables as environment/internal. That is still
	//   forbidden/impossible.

	return Variables{
		&VariableDefinition{
			Name: "IP_ADDRESS",
			CVOptions: CVOptions{
				Type:     CVTypeEnv,
				Internal: true,
			},
		},
		&VariableDefinition{
			Name: "DNS_RECORD_NAME",
			CVOptions: CVOptions{
				Type:     CVTypeEnv,
				Internal: true,
			},
		},
		&VariableDefinition{
			Name: "KUBE_COMPONENT_INDEX",
			CVOptions: CVOptions{
				Type:     CVTypeEnv,
				Internal: true,
			},
		},
		&VariableDefinition{
			Name: "KUBERNETES_CLUSTER_DOMAIN",
			CVOptions: CVOptions{
				Type:     CVTypeUser, // The user can override this
				Internal: true,
			},
		},
	}
}
