package configstore

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hpcloud/fissile/model"
)

const (
	// JobConfigFileExtension is the file extension for json configurations
	JobConfigFileExtension = ".json"

	jobConfigPrefix = ""
	jobConfigIndent = "    "
)

type jsonConfigWriterProvider struct {
	opinions *opinions
	allProps map[string]map[string]interface{}
}

func newJSONConfigWriterProvider(opinions *opinions, allProps map[string]map[string]interface{}) (*jsonConfigWriterProvider, error) {
	return &jsonConfigWriterProvider{
		opinions: opinions,
		allProps: allProps,
	}, nil
}

func (w *jsonConfigWriterProvider) WriteConfigs(roleManifest *model.RoleManifest, builder *Builder) error {

	outputPath := builder.targetLocation

	if err := os.RemoveAll(outputPath); err != nil && err != os.ErrNotExist {
		return err
	}

	for _, role := range roleManifest.Roles {
		if err := os.MkdirAll(filepath.Join(outputPath, role.Name), 0755); err != nil {
			return err
		}

		for _, job := range role.Jobs {
			config, err := initializeConfigJSON()

			// Get job information
			config["job"].(map[string]interface{})["name"] = role.Name

			var templates []map[string]string
			for _, roleJob := range role.Jobs {
				templates = append(templates, map[string]string{"name": roleJob.Name})
			}
			config["job"].(map[string]interface{})["templates"] = templates

			if err != nil {
				return err
			}

			properties, err := getPropertiesForJob(job, w.allProps[propSetDistinguisher(job)], w.opinions)
			if err != nil {
				return err
			}
			config["properties"] = properties

			// Write out the configuration
			jobPath := filepath.Join(outputPath, role.Name, job.Name+JobConfigFileExtension)
			jobJSON, err := json.MarshalIndent(config, jobConfigPrefix, jobConfigIndent)
			if err != nil {
				return err
			}
			if err = ioutil.WriteFile(jobPath, jobJSON, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// getPropertiesForJob returns the parameters for the given job, using its specs and opinions
func getPropertiesForJob(job *model.Job, allProps map[string]interface{}, opinions *opinions) (map[string]interface{}, error) {
	if allProps == nil {
		return map[string]interface{}{}, nil
	}
	props, err := deepCopy(allProps)
	if err != nil {
		return nil, err
	}

	// Get configs from the specs
	for _, property := range job.Properties {
		if err := insertConfig(props, property.Name, property.Default); err != nil {
			return nil, err
		}
	}

	// Get configs from the opinions
	for _, uniqueConfig := range job.Release.GetUniqueConfigs() {
		keyPieces, err := getKeyGrams(uniqueConfig.Name)
		if err != nil {
			return nil, err
		}

		// Add light opinions
		value := opinions.GetOpinionForKey(opinions.Light, keyPieces)
		if value != nil {
			if err := insertConfig(props, uniqueConfig.Name, value); err != nil {
				return nil, err
			}
		}

		// Subtract dark opinions
		value = opinions.GetOpinionForKey(opinions.Dark, keyPieces)
		if value != nil {
			deleteConfig(props, keyPieces, value)
		}
	}
	return props, nil
}

// initializeConfigJSON returns the scaffolding for the BOSH-style JSON structure
func initializeConfigJSON() (map[string]interface{}, error) {
	var config map[string]interface{}
	err := json.Unmarshal([]byte(`{
		"job": {
			"templates": []
		},
		"parameters": {},
		"properties": {},
		"networks": {
			"default": {}
		}
	}`), &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal initial config: %+v", err)
	}
	return config, nil
}

// deleteConfig removes a configuration value from the configuration map
func deleteConfig(config map[string]interface{}, keyPieces []string, value interface{}) {
	for _, key := range keyPieces[:len(keyPieces)-1] {
		child, ok := config[key].(map[string]interface{})
		if !ok {
			// If we can't find an ancestor key, we can't possibly find the target one.
			// That counts as having been deleted.
			return
		}
		config = child
	}
	lastKey := keyPieces[len(keyPieces)-1]
	if valueMap, ok := value.(map[interface{}]interface{}); ok {
		configMapDifference(config[lastKey].(map[string]interface{}), valueMap, keyPieces)
	} else {
		delete(config, lastKey)
	}
}

// configMapDifference removes entries in the second map from the first map
func configMapDifference(config map[string]interface{}, difference map[interface{}]interface{}, stack []string) error {
	for keyVal, value := range difference {
		key := keyVal.(string)
		if mapValue, ok := value.(map[interface{}]interface{}); ok {
			configValue := config[key]
			if configValueMap, ok := configValue.(map[string]interface{}); ok {
				if err := configMapDifference(configValueMap, mapValue, append(stack, key)); err != nil {
					return err
				}
			} else if configValue != nil {
				return fmt.Errorf("Attempting to descend into dark opinions key %s which is not a hash in the base configuration", strings.Join(append(stack, key), "."))
			}
		} else {
			delete(config, key)
		}
	}
	return nil
}
