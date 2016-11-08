package configstore

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/fissile/util"
)

const (
	// SpecStore is the prefix for the spec defaults keys
	SpecStore = "spec"
	// OpinionsStore is the prefix for the opinions defaults keys
	OpinionsStore = "opinions"

	// DescriptionsStore is the prefix for the description keys
	DescriptionsStore = "descriptions"

	// JSONProvider is the name of the JSON output provider; outputs one file per job
	JSONProvider = "json"
)

// Builder creates a base configuration to be fed into Consul or something similar
type Builder struct {
	provider          string
	lightOpinionsPath string
	darkOpinionsPath  string
	targetLocation    string
}

// NewConfigStoreBuilder creates a new configstore.Builder
func NewConfigStoreBuilder(provider, lightOpinionsPath, darkOpinionsPath, targetLocation string) *Builder {
	configStoreManager := &Builder{
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

	allProps, namesWithoutDefaults, err := getAllPropertiesForRoleManifest(roleManifest)
	if err != nil {
		return err
	}

	switch {
	case c.provider == JSONProvider:
		writer, err = newJSONConfigWriterProvider(opinions, allProps)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid config writer provider %s", c.provider)
	}

	if err := checkKeysInProperties(opinions.Light, allProps, namesWithoutDefaults, "light", os.Stderr); err != nil {
		return err
	}
	if err := checkKeysInProperties(opinions.Dark, allProps, namesWithoutDefaults, "dark", os.Stderr); err != nil {
		return err
	}

	if err := writer.WriteConfigs(roleManifest, c); err != nil {
		return err
	}

	return nil
}

// BoshKeyToConsulPath maps dotted names to slash-delimited names
func BoshKeyToConsulPath(key, store string) (string, error) {
	keyGrams, err := getKeyGrams(key)
	if err != nil {
		return "", err
	}

	keyGrams = append([]string{"", store}, keyGrams...)
	return strings.Join(keyGrams, "/"), nil
}

func propSetDistinguisher(job *model.Job) string {
	return job.Release.Name
}

// getAllPropertiesForRoleManifest returns all of the properties available from a role manifest's specs
// The return values props and names have types
// releaseName+":"+jobName => { property-name => property-value }
func getAllPropertiesForRoleManifest(roleManifest *model.RoleManifest) (map[string]map[string]interface{}, map[string]map[string]struct{}, error) {
	props := make(map[string]map[string]interface{})
	names := make(map[string]map[string]struct{})

	for _, role := range roleManifest.Roles {
		for _, job := range role.Jobs {
			releaseJobName := propSetDistinguisher(job)
			for _, property := range job.Properties {
				if _, ok := props[releaseJobName]; !ok {
					props[releaseJobName] = make(map[string]interface{})
				}
				if err := insertConfig(props[releaseJobName], property.Name, property.Default); err != nil {
					return nil, nil, err
				}
				if property.Default == nil {
					// Allow children of things with no defaults; they are empty hashes / lists.
					if _, ok := names[releaseJobName]; !ok {
						names[releaseJobName] = make(map[string]struct{})
					}
					names[releaseJobName][property.Name] = struct{}{}
				}
			}
		}
	}

	return props, names, nil
}

// checkKeysInProperties ensures that all opinons override values in props.
// The type of opinion (light or dark) is given in opinionName for messages.
// If any key exists in props with no default value, it is skipped as children are expected.
// Only the top level key being missing can generate errors; if any child is missing, they are emitted as
// warnings on warningWriter.
func checkKeysInProperties(opinions map[string]interface{}, props map[string]map[string]interface{}, namesWithoutDefaultsPerReleaseJob map[string]map[string]struct{}, opinionName string, warningWriter io.Writer) error {
	var results []string
	var warnings []string

	// Declare checkInner to capture itself in a closure so we can recurse
	var checkInner func(opinions map[interface{}]interface{}, props map[string]interface{}, namesWithoutDefaults map[string]struct{}, keyGramPrefix []string)

	checkInner = func(opinions map[interface{}]interface{}, props map[string]interface{}, namesWithoutDefaults map[string]struct{}, keyGramPrefix []string) {
		for key, value := range opinions {
			keyStr := key.(string)
			newKeyGramPrefix := append(keyGramPrefix, keyStr)

			// If a key has no defaults, ignore it and all descendents - it's a hash or list to be filled in.
			keyName := strings.Join(newKeyGramPrefix, ".")
			if _, ok := namesWithoutDefaults[keyName]; ok {
				continue
			}

			if opinionValue, ok := value.(map[interface{}]interface{}); ok {
				if paramValue, ok := props[keyStr].(map[string]interface{}); !ok {
					if len(newKeyGramPrefix) > 1 {
						warnings = append(warnings, keyName)
					} else {
						results = append(results, newKeyGramPrefix[0])
					}
				} else {
					checkInner(opinionValue, paramValue, namesWithoutDefaults, newKeyGramPrefix)
				}
			} else {
				if _, ok := props[keyStr]; !ok {
					if len(newKeyGramPrefix) > 1 {
						warnings = append(warnings, keyName)
					} else {
						results = append(results, newKeyGramPrefix[0])
					}
				}
			}
		}
	}

	// For the purposes of this function, we don't need to distinguish between releases
	flattenedProps := make(map[string]interface{})
	for _, releaseProps := range props {
		if err := util.JSONMergeBlobs(flattenedProps, releaseProps); err != nil {
			return err
		}
	}

	flattenedNamesWithoutDefaultsPerRelease := make(map[string]struct{})
	for _, releaseNamesWithoutDefaults := range namesWithoutDefaultsPerReleaseJob {
		for name := range releaseNamesWithoutDefaults {
			flattenedNamesWithoutDefaultsPerRelease[name] = struct{}{}
		}
	}

	properties, ok := opinions["properties"].(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("failed to load %s opinions from %+v", opinionName, opinions)
	}

	checkInner(properties, flattenedProps, flattenedNamesWithoutDefaultsPerRelease, []string{})

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
