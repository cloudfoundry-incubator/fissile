package kube

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"code.cloudfoundry.org/fissile/helm"
	"github.com/Masterminds/sprig"
	yaml "gopkg.in/yaml.v2"
)

// RenderNode renders a helm node given the configuration.
// The configuration may be nil, or map[string]interface{}
// If it is nil, default values are used.
// Otherwise, if the keys contains dots, they are interpreted as the paths
// to the elements to override.  If they do not contain dots, the map itself
// is considered the override.
func RenderNode(node helm.Node, config interface{}) ([]byte, error) {

	basicConfig, err := getBasicConfig()
	if err != nil {
		return nil, err
	}

	actualConfig := map[string]interface{}{
		"Values": basicConfig,
		"Capabilities": map[string]interface{}{
			"KubeVersion": map[string]interface{}{
				"Major": "1",
				"Minor": "8",
			},
		},
		"Template": map[string]interface{}{
			"BasePath": "",
		},
		"Chart": map[string]interface{}{
			"AppVersion": "1.22.333.4444",
			"Name":       "MyChart",
			"Version":    "42.1+foo",
		},
		"Release": map[string]interface{}{
			"Name":    "MyRelease",
			"Service": "Tiller",
		},
	}
	if overrides, ok := config.(map[string]interface{}); ok {
		for k, v := range overrides {
			actualConfig = mergeMap(actualConfig, v, 0, strings.Split(k, ".")...)
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
	functions["toYaml"] = renderToYaml

	// Note: Replicate helm's behaviour on missing keys.
	tmpl, err := template.New("").Option("missingkey=zero").Funcs(functions).Parse(string(helmConfig.Bytes()))

	if err != nil {
		//fmt.Printf("TEMPLATE PARSE FAIL: %s\n%s\nPARSE END\n", err, string(helmConfig.Bytes()))
		return nil, err
	}
	if err = tmpl.Execute(&yamlConfig, actualConfig); err != nil {
		//fmt.Printf("TEMPLATE EXEC FAIL\n%s\n%s\nEXEC END\n", string(helmConfig.Bytes()), err)
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
func mergeMap(obj map[string]interface{}, value interface{}, index int, key ...string) map[string]interface{} {
	if len(key) < 1 {
		panic("No keys")
	}
	if index > len(key) || index < 0 {
		panic(fmt.Sprintf("Invalid index %d in keys %v", index, key))
	}
	if index == len(key)-1 {
		// This will only work for untyped nil values
		if value == nil {
			delete(obj, key[index])
		} else {
			obj[key[index]] = value
		}
		return obj
	}
	if _, ok := obj[key[index]]; !ok {
		obj[key[index]] = make(map[string]interface{})
	}
	if _, ok := obj[key[index]].(map[string]interface{}); !ok {
		panic(fmt.Sprintf("Invalid object at %s: is not a map: %+v",
			strings.Join(key[:index], "."),
			obj[key[index]]))
	}
	obj[key[index]] = mergeMap(obj[key[index]].(map[string]interface{}), value, index+1, key...)
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
	return filepath.Base(name), nil
}

func renderToYaml(data interface{}) (string, error) {
	yml, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(yml)), nil
}

// getBasicConfig returns the built-in configuration
func getBasicConfig() (map[string]interface{}, error) {
	var convertNode func(node helm.Node, path []string) (interface{}, error)
	convertNode = func(node helm.Node, path []string) (interface{}, error) {
		switch n := node.(type) {
		case *helm.Scalar:
			var v interface{}
			err := yaml.Unmarshal([]byte(n.String()), &v)
			if err != nil {
				return nil, fmt.Errorf("Error parsing node at %s: %s", strings.Join(path, "."), err)
			}
			return v, nil
		case *helm.List:
			var values []interface{}
			for i, v := range n.Values() {
				converted, err := convertNode(v, append(path, fmt.Sprintf("%d", i)))
				if err != nil {
					return nil, err
				}
				values = append(values, converted)
			}
			return values, nil
		case *helm.Mapping:
			values := make(map[string]interface{}, len(n.Names()))
			for _, k := range n.Names() {
				converted, err := convertNode(n.Get(k), append(path, k))
				if err != nil {
					return nil, err
				}
				values[k] = converted
			}
			return values, nil
		default:
			return nil, fmt.Errorf("Invalid node type at %s", strings.Join(path, "."))
		}
	}

	converted, err := convertNode(MakeBasicValues(), nil)
	if err != nil {
		return nil, err
	}
	return converted.(map[string]interface{}), nil
}
