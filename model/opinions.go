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

// FlatMap converts the incoming nested map of opinions into a flat
// map of properties to values (strings).  Things of note: The
// toplevel map of "properties" has to be skipped.  This common prefix
// is NOT part of the property key / namespace.
func FlatMap(opinions map[string]interface{}) map[string]string {
	return flatMapString(make(map[string]string), "", opinions, 1)
}

// The following pair of functions differs (only) in the type of the
// incoming "opinions". The treatment is 99% the same, with the only
// difference of the Interface variant using Sprintf to convert the
// key to a string, which it already is for String. The identical
// parts of both are factored into "flatMapRecurse".
//
// And while we seem to need the String variant only for the toplevel
// map, as everything below looks to be Interface only, I don't see
// how to get rid of it either, as a "map[string]interface{}" value
// cannot be given to a "map[interface{}]interface{}" argument.

func flatMapString(result map[string]string, prefix string, opinions map[string]interface{}, skip int) map[string]string {
	for ks, value := range opinions {
		// Here the `ks` iteration variable is already a
		// string, contrary to flatMapI below.

		result = flatMapRecurse(result, prefix, ks, value, skip)
	}
	return result
}

func flatMapInterface(result map[string]string, prefix string, opinions map[interface{}]interface{}, skip int) map[string]string {
	for key, value := range opinions {
		ks := fmt.Sprintf("%v", key)
		// Generate string iteration variable from general
		// key, compare flatMapS above.

		result = flatMapRecurse(result, prefix, ks, value, skip)
	}
	return result
}

func flatMapRecurse(result map[string]string, prefix, ks string, value interface{}, skip int) map[string]string {
	// 'c' for child
	var cprefix string
	var cskip int

	if skip > 0 {
		cprefix = prefix
		cskip = skip - 1
	} else {
		cprefix = prefix + ks + "."
		cskip = skip
	}

	if vmap, ok := value.(map[string]interface{}); ok {
		return flatMapString(result, cprefix, vmap, cskip)
	}
	if vmap, ok := value.(map[interface{}]interface{}); ok {
		return flatMapInterface(result, cprefix, vmap, cskip)
	}
	if skip == 0 {
		result[prefix+ks] = fmt.Sprintf("%v", value)
	}
	return result
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
