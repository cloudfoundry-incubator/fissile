package model

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type opinions struct {
	Light map[string]interface{}
	Dark  map[string]interface{}
}

// newOpinions returns the json opinions for the light and dark opinion files
func newOpinions(lightFile, darkFile string) (*opinions, error) {
	result := &opinions{}

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

func (o *opinions) GetOpinionForKey(opinions map[string]interface{}, keyPieces []string) (result interface{}) {
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
