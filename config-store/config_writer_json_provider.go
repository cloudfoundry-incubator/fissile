package configstore

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hpcloud/fissile/model"
	"github.com/termie/go-shutil"
)

type jsonConfigWriterProvider struct {
	tempDir string
	prefix  string
}

func newJSONConfigWriterProvider(prefix string) (*jsonConfigWriterProvider, error) {
	tempDir, err := ioutil.TempDir("", "fissile-config-writer-json")
	if err != nil {
		return nil, err
	}

	return &jsonConfigWriterProvider{
		tempDir: tempDir,
		prefix:  prefix,
	}, nil
}

func (w *jsonConfigWriterProvider) WriteConfigs(role *model.Role, job *model.Job, builder *Builder) error {

	if err := os.MkdirAll(filepath.Join(w.tempDir, w.prefix, role.Name), 0755); err != nil && err != os.ErrExist {
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

		// Get configs from the specs
		if err != nil {
			return err
		}

		params := config["parameters"].(map[string]interface{})
		for _, property := range job.Properties {
			if err := insertConfig(params, property.Name, property.Default); err != nil {
				return err
			}
		}

		// Get configs from the opinions
		opinions, err := newOpinions(builder.lightOpinionsPath, builder.darkOpinionsPath)
		if err != nil {
			return err
		}
		for _, uniqueConfig := range job.Release.GetUniqueConfigs() {
			keyPieces, err := getKeyGrams(uniqueConfig.Name)
			if err != nil {
				return err
			}
			masked, value := opinions.GetOpinionForKey(keyPieces)
			if masked {
				deleteConfig(params, uniqueConfig.Name)
				continue
			}
			if value == nil {
				continue
			}
			if err := insertConfig(params, uniqueConfig.Name, value); err != nil {
				return err
			}
		}

		// Write out the configuration
		jobPath := filepath.Join(w.tempDir, w.prefix, role.Name, job.Name+".json")
		jobJSON, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		if err = ioutil.WriteFile(jobPath, jobJSON, 0644); err != nil {
			return err
		}
	}

	return nil
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

// insertConfig adds a configuration value into the configuration map
func insertConfig(config map[string]interface{}, name string, value interface{}) error {
	keyPieces, err := getKeyGrams(name)
	if err != nil {
		return err
	}

	parent := config
	for _, key := range keyPieces[:len(keyPieces)-1] {
		if child, ok := parent[key].(map[string]interface{}); ok {
			parent = child
		} else {
			child = make(map[string]interface{})
			parent[key] = child
			parent = child
		}
	}
	parent[keyPieces[len(keyPieces)-1]] = valueToJSONable(value)
	return nil
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
			return nil
		}
		config = child
	}
	delete(config, keyPieces[len(keyPieces)-1])
	return nil
}

// Save commits the configuration to the target path
func (w *jsonConfigWriterProvider) Save(targetPath string) error {
	if err := os.RemoveAll(targetPath); err != nil && err != os.ErrNotExist {
		return err
	}
	return shutil.CopyTree(w.tempDir, targetPath, &shutil.CopyTreeOptions{
		Symlinks:     true,
		CopyFunction: shutil.Copy,
	})
}

func (w *jsonConfigWriterProvider) CleanUp() error {
	if err := os.RemoveAll(w.tempDir); err != nil && err != os.ErrNotExist {
		return err
	}
	return nil
}
