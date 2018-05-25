package testhelpers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/SUSE/fissile/helm"

	"github.com/Masterminds/sprig"
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
		"Capabilities": map[string]interface{}{
			"KubeVersion": map[string]interface{}{
				"Major": "1",
				"Minor": "8",
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

	// The functions added here are implementations of the helm
	// functions used by fissile-generated templates. While we get
	// most of them from sprig we need two which are implemented
	// by helm itself. We provide fakes.

	functions := sprig.TxtFuncMap()
	functions["include"] = renderInclude
	functions["required"] = renderRequired

	tmpl, err := template.New("").Funcs(functions).Parse(string(helmConfig.Bytes()))

	if err != nil {
		//fmt.Printf("TEMPLATE PARSE FAIL\n%s\nPARSE END\n", string(helmConfig.Bytes()))
		return nil, err
	}
	if err = tmpl.Execute(&yamlConfig, actualConfig); err != nil {
		fmt.Printf("TEMPLATE EXEC FAIL\n%s\nEXEC END\n", string(helmConfig.Bytes()))
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

// RoundtripKube serializes and then unserializes a helm node without
// performing any type of template resolution. As such the
// unserialization step will only work if the helm node has no
// templating (blocks), i.e. is destined for a kube output.
func RoundtripKube(node helm.Node) (interface{}, error) {
	var yamlConfig bytes.Buffer

	if err := helm.NewEncoder(&yamlConfig).Encode(node); err != nil {
		return nil, err
	}

	//fmt.Printf("YAML FAIL\n%s\nEXEC END\n", string(yamlConfig.Bytes()))

	var actual interface{}
	if err := yaml.Unmarshal(yamlConfig.Bytes(), &actual); err != nil {
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

// Helper functions for the template engine. Semi-snarfed from helm
// for our testing. Avoid vendoring of the whole helm rendering
// engine.

// RenderEncodeBase64 provides easy base64 encoding for strings.
func RenderEncodeBase64(in string) string {
	return base64.StdEncoding.EncodeToString([]byte(in))
}

func renderRequired(msg string, v interface{}) (interface{}, error) {
	if v == nil {
		return v, fmt.Errorf(msg)
	} else if _, ok := v.(string); ok {
		if v == "" {
			return v, fmt.Errorf(msg)
		}
	}
	return v, nil
}

func renderInclude(name string, data interface{}) (string, error) {
	// Fake include -- Actually implementing this function would
	// require adding the handling of `associated` templates.  A
	// first run at this generated a stack overflow.  The fake
	// simply shows what path/name would have been included.
	base := filepath.Base(name)
	return base, nil
}
