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
	flattenOpinionsString(result, "", opinions)
	return result
}

// The following pair of functions differs (only) in the type of the
// incoming "opinions". The treatment is 99% the same, with the only
// difference of the Interface variant using Sprintf to convert the
// key to a string, which it already is for String. The identical
// parts of both are factored into "flattenOpinionsRecurse".
//
// And while we seem to need the String variant only for the toplevel
// map, as everything below looks to be Interface only, I don't see
// how to get rid of it either, as a "map[string]interface{}" value
// cannot be given to a "map[interface{}]interface{}" argument.

func flattenOpinionsString(result map[string]string, prefix string, opinions map[string]interface{}) {
	for ks, value := range opinions {
		// Here the `ks` iteration variable is already a
		// string, contrary to flattenOpinionsInterface below.
		flattenOpinionsRecurse(result, prefix, ks, value)
	}
}

func flattenOpinionsInterface(result map[string]string, prefix string, opinions map[interface{}]interface{}) {
	for key, value := range opinions {
		ks := fmt.Sprintf("%v", key)
		// Generate string iteration variable from general
		// key, compare flattenOpinionsString above.
		flattenOpinionsRecurse(result, prefix, ks, value)
	}
}

func flattenOpinionsRecurse(result map[string]string, prefix, ks string, value interface{}) {
	// 'c' for child
	cprefix := prefix + ks + "."

	if vmap, ok := value.(map[string]interface{}); ok {
		flattenOpinionsString(result, cprefix, vmap)
		return
	}
	if vmap, ok := value.(map[interface{}]interface{}); ok {
		flattenOpinionsInterface(result, cprefix, vmap)
		return
	}

	result[prefix+ks] = fmt.Sprintf("%v", value)
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
