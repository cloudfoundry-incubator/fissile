package configstore

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/hpcloud/fissile/model"
)

const (
	// SpecStore is the prefix for the spec defaults keys
	SpecStore = "spec"
	// OpinionsStore is the prefix for the opinions defaults keys
	OpinionsStore = "opinions"

	// DescriptionsStore is the prefix for the description keys
	DescriptionsStore = "descriptions"

	// DirTreeProvider is the name of the default config writer that creates a filesystem tree
	DirTreeProvider = "dirtree"

	// JSONProvider is the name of the JSON output provider; outputs one file per job
	JSONProvider = "json"
)

// Builder creates a base configuration to be fed into Consul or something similar
type Builder struct {
	prefix            string
	provider          string
	lightOpinionsPath string
	darkOpinionsPath  string
	targetLocation    string
}

// NewConfigStoreBuilder creates a new configstore.Builder
func NewConfigStoreBuilder(prefix, provider, lightOpinionsPath, darkOpinionsPath, targetLocation string) *Builder {
	configStoreManager := &Builder{
		prefix:            prefix,
		provider:          provider,
		lightOpinionsPath: lightOpinionsPath,
		darkOpinionsPath:  darkOpinionsPath,
		targetLocation:    targetLocation,
	}

	return configStoreManager
}

// WriteBaseConfig generates the configuration base from a role manifest
func (c *Builder) WriteBaseConfig(roleManifest *model.RoleManifest) error {
	var writer configWriter
	var err error

	opinions, err := newOpinions(c.lightOpinionsPath, c.darkOpinionsPath)
	if err != nil {
		return err
	}

	allParams, err := getAllParamsForRoleManifest(roleManifest)
	if err != nil {
		return err
	}

	switch {
	case c.provider == DirTreeProvider:
		writer, err = newDirTreeConfigWriterProvider(opinions, allParams)
		if err != nil {
			return err
		}
	case c.provider == JSONProvider:
		writer, err = newJSONConfigWriterProvider(opinions, allParams)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid config writer provider %s", c.provider)
	}

	if err := checkKeysInParams(opinions.Light, allParams, "light", os.Stderr); err != nil {
		return err
	}
	if err := checkKeysInParams(opinions.Dark, allParams, "dark", os.Stderr); err != nil {
		return err
	}

	if err := writer.WriteConfigs(roleManifest, c); err != nil {
		return err
	}

	return nil
}

func (c *Builder) boshKeyToConsulPath(key, store string) (string, error) {
	keyGrams, err := getKeyGrams(key)
	if err != nil {
		return "", err
	}

	keyGrams = append([]string{"", c.prefix, store}, keyGrams...)
	return strings.Join(keyGrams, "/"), nil
}

// getAllParamsForRoleManifest returns all of the parameters available from a role manifest's specs
func getAllParamsForRoleManifest(roleManifest *model.RoleManifest) (map[string]interface{}, error) {
	params := make(map[string]interface{})

	for _, role := range roleManifest.Roles {
		for _, job := range role.Jobs {
			for _, property := range job.Properties {
				if err := insertConfig(params, property.Name, property.Default); err != nil {
					return nil, err
				}
			}
		}
	}

	return params, nil
}

// checkKeysInParams ensures that all opinons override values in params.
// The type of opinion (light or dark) is given in opinionName for messages.
// Only the top level key being missing can generate errors; if any child is missing (e.g. uaa.clients),
// they are emitted as warnings on warningWriter.
func checkKeysInParams(opinions, params map[string]interface{}, opinionName string, warningWriter io.Writer) error {
	var results []string
	var warnings []string

	// Declare checkInner to capture itself in a closure so we can recurse
	var checkInner func(opinions map[interface{}]interface{}, params map[string]interface{}, keyGramPrefix []string)

	checkInner = func(opinions map[interface{}]interface{}, params map[string]interface{}, keyGramPrefix []string) {
		for key, value := range opinions {
			keyStr := key.(string)
			newKeyGramPrefix := append(keyGramPrefix, keyStr)
			if opinionValue, ok := value.(map[interface{}]interface{}); ok {
				if paramValue, ok := params[keyStr].(map[string]interface{}); !ok {
					if len(newKeyGramPrefix) > 1 {
						warnings = append(warnings, strings.Join(newKeyGramPrefix, "."))
					} else {
						results = append(results, newKeyGramPrefix[0])
					}
				} else {
					checkInner(opinionValue, paramValue, newKeyGramPrefix)
				}
			} else {
				if _, ok := params[keyStr]; !ok {
					if len(newKeyGramPrefix) > 1 {
						warnings = append(warnings, strings.Join(newKeyGramPrefix, "."))
					} else {
						results = append(results, newKeyGramPrefix[0])
					}
				}
			}
		}
	}

	properties, ok := opinions["properties"].(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("failed to load %s opinions from %+v", opinionName, opinions)
	}
	checkInner(properties, params, []string{})

	if len(results) > 0 {
		indent := "\n    "
		sort.Strings(results)
		return fmt.Errorf("extra top level keys in %s opinions:%s%s", opinionName, indent, strings.Join(results, indent))
	}

	if len(warnings) > 0 && warningWriter != nil {
		sort.Strings(warnings)
		fmt.Fprintf(warningWriter, "Found %v orphaned configs in %s opinions:\n", len(warnings), opinionName)
		for _, warning := range warnings {
			fmt.Fprintf(warningWriter, "    %s\n", warning)
		}
	}

	return nil
}
