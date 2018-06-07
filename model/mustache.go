package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SUSE/fissile/mustache"
)

// MakeMapOfVariables converts the sequence of configuration variables
// into a map we can manipulate more directly by name.
func MakeMapOfVariables(roleManifest *RoleManifest) CVMap {
	configsDictionary := CVMap{}

	for _, config := range roleManifest.Configuration.Variables {
		configsDictionary[config.Name] = config
	}

	for _, config := range builtins() {
		configsDictionary[config.Name] = config
	}

	return configsDictionary
}

// GetVariablesForRole returns all the environment variables required for
// calculating all the templates for the role
func (r *Role) GetVariablesForRole() (ConfigurationVariableSlice, error) {

	configsDictionary := MakeMapOfVariables(r.roleManifest)

	configs := CVMap{}

	// First, render all referenced variables of type user.

	for _, roleJob := range r.RoleJobs {
		for _, property := range roleJob.Properties {
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
						if confVar.Type == CVTypeUser {
							configs[confVar.Name] = confVar
						} else if confVar.Type == CVTypeEnv && confVar.Default != "" {
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
		if confVar.Type == CVTypeUser && confVar.Internal {
			configs[confVar.Name] = confVar
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

func builtins() ConfigurationVariableSlice {
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

	return ConfigurationVariableSlice{
		&ConfigurationVariable{
			Name:     "IP_ADDRESS",
			Type:     CVTypeEnv,
			Internal: true,
		},
		&ConfigurationVariable{
			Name:     "DNS_RECORD_NAME",
			Type:     CVTypeEnv,
			Internal: true,
		},
		&ConfigurationVariable{
			Name:     "KUBE_COMPONENT_INDEX",
			Type:     CVTypeEnv,
			Internal: true,
		},
		&ConfigurationVariable{
			Name:     "KUBE_SERVICE_DOMAIN_SUFFIX",
			Type:     CVTypeUser, // The user can override this
			Internal: true,
		},
		&ConfigurationVariable{
			Name:     "KUBERNETES_CLUSTER_DOMAIN",
			Type:     CVTypeUser, // The user can override this
			Internal: true,
		},
	}
}
