package model

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Opinions holds the light and dark opinions given to fissile
type Opinions struct {
	Light map[string]interface{}
	Dark  map[string]interface{}
}

// NewOpinions returns the json opinions for the light and dark opinion files
func NewOpinions(lightFile, darkFile string) (*Opinions, error) {
	result := &Opinions{}

	manifestContents, err := ioutil.ReadFile(lightFile)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(manifestContents), &result.Light)
	if err != nil {
		return nil, err
	}

	manifestContents, err = ioutil.ReadFile(darkFile)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(manifestContents), &result.Dark)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FlattenOpinions converts the incoming nested map of opinions into a flat
// map of properties to values (strings).
func FlattenOpinions(opinions map[string]interface{}) map[string]string {
	result := make(map[string]string)
	flattenOpinionsRecurse(result, "", opinions)
	return result
}

func flattenOpinionsRecurse(result map[string]string, prefix string, value interface{}) {

	var cprefix string
	if prefix == "" {
		cprefix = prefix
	} else {
		cprefix = prefix + "."
	}

	if vmap, ok := value.(map[string]interface{}); ok {
		for ks, value := range vmap {
			// Here the `ks` iteration variable is already a
			// string, contrary to the Interface loop below.
			flattenOpinionsRecurse(result, cprefix+ks, value)
		}
		return
	}
	if vmap, ok := value.(map[interface{}]interface{}); ok {
		for key, value := range vmap {
			ks := fmt.Sprintf("%v", key)
			// Generate string iteration variable from general
			// key, compare String loop above.
			flattenOpinionsRecurse(result, cprefix+ks, value)
		}
		return
	}

	result[prefix] = fmt.Sprintf("%v", value)
}

// GetOpinionForKey pulls an opinion out of the holding container.
func (o *Opinions) GetOpinionForKey(opinions map[string]interface{}, keyPieces []string) (result interface{}) {
	return getDeepValueFromManifest(opinions, keyPieces)
}

func getDeepValueFromManifest(manifest map[string]interface{}, keyPieces []string) (result interface{}) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
		}
	}()

	var value interface{}
	var hasKey bool

	if properties, ok := manifest["properties"]; ok {
		for _, keyPiece := range keyPieces {
			value, hasKey = properties.(map[interface{}]interface{})[keyPiece]

			if !hasKey {
				return nil
			}

			properties = value
		}
	}

	return value
}
