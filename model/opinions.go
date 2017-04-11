package model

import (
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
