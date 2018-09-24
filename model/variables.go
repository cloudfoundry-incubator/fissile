package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Variables from the BOSH manifests variables section
type Variables []*VariableDefinition

// VariableDefinition from the BOSH deployment manifest
// Type is used to decide on a generator
type VariableDefinition struct {
	Name      string
	Type      string
	Options   VariableOptions
	CVOptions CVOptions
}

// VariableOptions are not structured, their content depends on the type
type VariableOptions map[string]interface{}

// CVOptions is a configuration to be exposed to the IaaS
//
// Notes on the fields Type and Internal.
// 1. Type's legal values are `user` and `environment`.
//    `user` is default.
//
//    A `user` CV is rendered into k8s yml config files, etc. to make it available to roles who need it.
//    - An internal CV is rendered to all roles.
//    - A public CV is rendered only to the roles whose templates refer to the CV.
//
//    An `environment` CV comes from a script, not the user. Being
//    internal this way it is not rendered to any configuration files.
//
// 2. Internal's legal values are all YAML boolean values.
//    A public CV is used in templates
//    An internal CV is not, consumed in a script instead.
type CVOptions struct {
	PreviousNames []string    `yaml:"previous_names"`
	Default       interface{} `yaml:"default"`
	Description   string      `yaml:"description"`
	Example       string      `yaml:"example"`
	Type          CVType      `yaml:"type"`
	Internal      bool        `yaml:"internal,omitempty"`
	Secret        bool        `yaml:"secret,omitempty"`
	Required      bool        `yaml:"required,omitempty"`
	Immutable     bool        `yaml:"immutable,omitempty"`
}

// CVType is the type of the configuration variable; see the constants below
type CVType string

const (
	// CVTypeUser is for user-specified variables (default)
	CVTypeUser = CVType("user")
	// CVTypeEnv is for script-specified variables
	CVTypeEnv = CVType("environment")
)

// CVMap is a map from variable name to ConfigurationVariable, for
// various places which require quick access/search/existence check.
type CVMap map[string]*VariableDefinition

type internalVariable struct {
	CVOptions CVOptions `yaml:"options"`
}

type internalVariableDefinitions struct {
	Variables []internalVariable `yaml:"variables"`
}

// Value fetches the value of config variable
func (config *VariableDefinition) Value() (bool, string) {
	var value interface{}

	value = config.CVOptions.Default

	if value == nil {
		return false, ""
	}

	var stringifiedValue string
	if valueAsString, ok := value.(string); ok {
		var err error
		stringifiedValue, err = strconv.Unquote(fmt.Sprintf(`"%s"`, valueAsString))
		if err != nil {
			stringifiedValue = valueAsString
		}
	} else {
		asJSON, _ := json.Marshal(value)
		stringifiedValue = string(asJSON)
	}

	return true, stringifiedValue
}

// Len is the number of ConfigurationVariables in the slice
func (confVars Variables) Len() int {
	return len(confVars)
}

// Less reports whether config variable at index i sort before the one at index j
func (confVars Variables) Less(i, j int) bool {
	return strings.Compare(confVars[i].Name, confVars[j].Name) < 0
}

// Swap exchanges configuration variables at index i and index j
func (confVars Variables) Swap(i, j int) {
	confVars[i], confVars[j] = confVars[j], confVars[i]
}
