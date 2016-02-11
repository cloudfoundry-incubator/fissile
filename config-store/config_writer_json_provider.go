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
	jobConfigFileExtension = ".json"
	jobConfigPrefix        = ""
	jobConfigIndent        = "    "
)

type jsonConfigWriterProvider struct {
	opinions  *opinions
	allParams map[string]interface{}
}

type errConfigNotExist struct {
	error
}

func newErrorConfigNotExist(key string) error {
	return &errConfigNotExist{
		fmt.Errorf("The configuration key %s does not exist", key),
	}
}

func newJSONConfigWriterProvider(opinions *opinions, allParams map[string]interface{}) (*jsonConfigWriterProvider, error) {
	return &jsonConfigWriterProvider{
		opinions:  opinions,
		allParams: allParams,
	}, nil
}

func (w *jsonConfigWriterProvider) WriteConfigs(roleManifest *model.RoleManifest, builder *Builder) error {

	outputPath := filepath.Join(builder.targetLocation, builder.prefix)

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

			params, err := getParamsForJob(job, w.allParams, w.opinions)
			if err != nil {
				return err
			}
			config["parameters"] = params

			// Write out the configuration
			jobPath := filepath.Join(outputPath, role.Name, job.Name+jobConfigFileExtension)
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

// getParamsForJob returns the parameters for the give job, using its specs and opinions
func getParamsForJob(job *model.Job, allParams map[string]interface{}, opinions *opinions) (map[string]interface{}, error) {
	params, err := deepCopy(allParams)
	if err != nil {
		return nil, err
	}

	// Get configs from the specs
	for _, property := range job.Properties {
		if err := insertConfig(params, property.Name, property.Default); err != nil {
			return nil, err
		}
	}

	// Get configs from the opinions
	for _, uniqueConfig := range job.Release.GetUniqueConfigs() {
		keyPieces, err := getKeyGrams(uniqueConfig.Name)
		if err != nil {
			return nil, err
		}
		masked, value := opinions.GetOpinionForKey(keyPieces)
		if masked {
			if err = deleteConfig(params, uniqueConfig.Name); err != nil {
				if _, ok := err.(*errConfigNotExist); ok {
					// Some keys, like uaa.client.*, only have the top level :|
					topLevelKey := strings.SplitN(uniqueConfig.Name, ".", 2)[0]
					if _, ok = params[topLevelKey]; !ok {
						// If the top level key is missing too, it's a hard error
						return nil, err
					}
				} else {
					return nil, err
				}
			}
			continue
		}
		if value == nil {
			continue
		}
		if err := insertConfig(params, uniqueConfig.Name, value); err != nil {
			return nil, err
		}
	}
	return params, nil
}

// initializeConfigJSON returns the scaffolding for the BOSH-style JSON structure
func initializeConfigJSON() (map[string]interface{}, error) {
	var config map[string]interface{}
	err := json.Unmarshal([]byte(`{
		"job": {
			"templates": []
		},
		"parameters": {},
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
func deleteConfig(config map[string]interface{}, name string) error {
	keyPieces, err := getKeyGrams(name)
	if err != nil {
		return err
	}

	for _, key := range keyPieces[:len(keyPieces)-1] {
		child, ok := config[key].(map[string]interface{})
		if !ok {
			return newErrorConfigNotExist(name)
		}
		config = child
	}
	delete(config, keyPieces[len(keyPieces)-1])
	return nil
}
