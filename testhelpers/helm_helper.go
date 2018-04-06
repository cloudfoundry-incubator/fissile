package testhelpers

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/SUSE/fissile/helm"

	"gopkg.in/yaml.v2"
)

// RenderNode renders a helm node given the configuration.
// The configuration may be nil, or map[string]interface{}
// If it is nil, default values are used.
// Otherwise, if the keys contains dots, they are interpreted as the paths
// to the elements to override.  If they do not contain dots, the map itself
// is considered the override.
func RenderNode(node helm.Node, config interface{}) ([]byte, error) {
	actualConfig := map[string]interface{}{
		"Values": map[string]interface{}{
			"kube": map[string]interface{}{
				"hostpath_available": true,
			},
		},
	}
	if overrides, ok := config.(map[string]interface{}); ok {
		for k, v := range overrides {
			actualConfig = mergeMap(actualConfig, v, strings.Split(k, ".")...)
		}
	} else if config != nil {
		return nil, fmt.Errorf("Invalid config %+v", config)
	}

	var helmConfig, yamlConfig bytes.Buffer

	if err := helm.NewEncoder(&helmConfig).Encode(node); err != nil {
		return nil, err
	}
	tmpl, err := template.New("").Parse(string(helmConfig.Bytes()))
	if err != nil {
		return nil, err
	}
	if err = tmpl.Execute(&yamlConfig, actualConfig); err != nil {
		return nil, err
	}
	return yamlConfig.Bytes(), nil
}

// RoundtripNode serializes and then unserializes a helm node.  The config
// override is identical to RenderNode().
func RoundtripNode(node helm.Node, config interface{}) (interface{}, error) {
	actualBytes, err := RenderNode(node, config)
	if err != nil {
		return nil, err
	}

	var actual interface{}
	if err := yaml.Unmarshal(actualBytes, &actual); err != nil {
		return nil, err
	}
	return actual, nil
}

// mergeMap returns the input map, but with an override applied.  An override
// is a key path and a value to replace with.
func mergeMap(obj map[string]interface{}, value interface{}, key ...string) map[string]interface{} {
	if len(key) < 1 {
		panic("No keys")
	}
	if len(key) == 1 {
		obj[key[0]] = value
		return obj
	}
	if _, ok := obj[key[0]]; !ok {
		obj[key[0]] = make(map[string]interface{})
	}
	obj[key[0]] = mergeMap(obj[key[0]].(map[string]interface{}), value, key[1:]...)
	return obj
}
