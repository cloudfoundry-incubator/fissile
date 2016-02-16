package configstore

import (
	"encoding/json"
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

type keyHash map[string]string

// HashDiffs summarizes the diffs between the two configs
type HashDiffs struct {
	AddedKeys     []string
	DeletedKeys   []string
	ChangedValues map[string][2]string
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

	allProps, err := getAllPropertiesForRoleManifest(roleManifest)
	if err != nil {
		return err
	}

	switch {
	case c.provider == DirTreeProvider:
		writer, err = newDirTreeConfigWriterProvider(opinions, allProps)
		if err != nil {
			return err
		}
	case c.provider == JSONProvider:
		writer, err = newJSONConfigWriterProvider(opinions, allProps)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid config writer provider %s", c.provider)
	}

	if err := checkKeysInProperties(opinions.Light, allProps, "light", os.Stderr); err != nil {
		return err
	}
	if err := checkKeysInProperties(opinions.Dark, allProps, "dark", os.Stderr); err != nil {
		return err
	}

	if err := writer.WriteConfigs(roleManifest, c); err != nil {
		return err
	}

	return nil
}

// BoshKeyToConsulPath maps dotted names to slash-delimited names
func BoshKeyToConsulPath(key, store, prefix string) (string, error) {
	keyGrams, err := getKeyGrams(key)
	if err != nil {
		return "", err
	}

	keyGrams = append([]string{"", prefix, store}, keyGrams...)
	return strings.Join(keyGrams, "/"), nil
}

// getAllPropertiesForRoleManifest returns all of the properties available from a role manifest's specs
func getAllPropertiesForRoleManifest(roleManifest *model.RoleManifest) (map[string]interface{}, error) {
	props := make(map[string]interface{})

	for _, role := range roleManifest.Roles {
		for _, job := range role.Jobs {
			for _, property := range job.Properties {
				if err := insertConfig(props, property.Name, property.Default); err != nil {
					return nil, err
				}
			}
		}
	}

	return props, nil
}

// checkKeysInProperties ensures that all opinons override values in props.
// The type of opinion (light or dark) is given in opinionName for messages.
// Only the top level key being missing can generate errors; if any child is missing (e.g. uaa.clients),
// they are emitted as warnings on warningWriter.
func checkKeysInProperties(opinions, props map[string]interface{}, opinionName string, warningWriter io.Writer) error {
	var results []string
	var warnings []string

	// Declare checkInner to capture itself in a closure so we can recurse
	var checkInner func(opinions map[interface{}]interface{}, props map[string]interface{}, keyGramPrefix []string)

	checkInner = func(opinions map[interface{}]interface{}, props map[string]interface{}, keyGramPrefix []string) {
		for key, value := range opinions {
			keyStr := key.(string)
			newKeyGramPrefix := append(keyGramPrefix, keyStr)
			if opinionValue, ok := value.(map[interface{}]interface{}); ok {
				if paramValue, ok := props[keyStr].(map[string]interface{}); !ok {
					if len(newKeyGramPrefix) > 1 {
						warnings = append(warnings, strings.Join(newKeyGramPrefix, "."))
					} else {
						results = append(results, newKeyGramPrefix[0])
					}
				} else {
					checkInner(opinionValue, paramValue, newKeyGramPrefix)
				}
			} else {
				if _, ok := props[keyStr]; !ok {
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
	checkInner(properties, props, []string{})

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

// DiffConfigurations calculates the differences in configs across two releases.
func (c *Builder) DiffConfigurations(releasePath1, releasePath2 string) (*HashDiffs, error) {
	var err error
	releases := [2]*model.Release{}
	releases[0], err = model.NewRelease(releasePath1)
	if err != nil {
		return nil, fmt.Errorf("Error loading release information for path %s: %s", releasePath1, err.Error())
	}
	releases[1], err = model.NewRelease(releasePath2)
	if err != nil {
		return nil, fmt.Errorf("Error loading release information for path %s: %s", releasePath2, err.Error())
	}
	hashes := [2]keyHash{keyHash{}, keyHash{}}
	// hashes := [2]{map[string]string{}, map[string]string{}}
	for idx, release := range releases {
		configs := release.GetUniqueConfigs()
		// Get the descriptions (do we care?)
		for _, config := range configs {
			key, err := BoshKeyToConsulPath(config.Name, DescriptionsStore, c.prefix)
			if err != nil {
				return nil, fmt.Errorf("Error getting config %s for release path %s: %s", config.Name, releasePath1, err.Error())
			}
			hashes[idx][key] = config.Description
		}
		// Get the spec configs
		for _, job := range release.Jobs {
			for _, property := range job.Properties {
				key, err := BoshKeyToConsulPath(fmt.Sprintf("%s.%s.%s", release.Name, job.Name, property.Name), SpecStore, c.prefix)
				if err != nil {
					return nil, err
				}
				hashes[idx][key], err = stringify(property.Default)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return c.compareHashes(hashes[0], hashes[1]), nil
}

func stringify(v interface{}) (string, error) {
	switch v1 := v.(type) {
	case int, uint:
		return fmt.Sprintf("%d", v1), nil
	case nil:
		return "", nil
	case bool:
		if v1 {
			return "TRUE", nil
		}
		return "FALSE", nil
	case string:
		return v1, nil
	default:
		bSlice, err := json.Marshal(v)
		if err == nil {
			return string(bSlice), err
		}
		res := fmt.Sprintf("%v", v)
		if res != "" {
			return res, nil
		}
		return string(bSlice), err
	}
}

func (c *Builder) compareHashes(v1Hash, v2Hash keyHash) *HashDiffs {
	changed := map[string][2]string{}
	deleted := []string{}
	added := []string{}

	for k, v := range v1Hash {
		v2, ok := v2Hash[k]
		if !ok {
			deleted = append(deleted, k)
		} else if v != v2 {
			changed[k] = [2]string{v, v2}
		}
	}
	for k := range v2Hash {
		_, ok := v1Hash[k]
		if !ok {
			added = append(added, k)
		}
	}
	return &HashDiffs{AddedKeys: added, DeletedKeys: deleted, ChangedValues: changed}
}
