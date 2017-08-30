package kube

import (
	"fmt"
	"io"
	"strings"
)

// ConfigType is the interface implemented by all config node types
type ConfigType interface {
	getComment() string
	getCondition() string
	setComment(string)
	setCondition(string)
	write(w io.Writer, prefix string)
}

func writeConditionAndComment(w io.Writer, config ConfigType, prefix *string, indent int, writeValue func()) {
	if strings.HasSuffix(*prefix, ":") {
		fmt.Fprintln(w, *prefix)
		*prefix = strings.Repeat(" ", strings.LastIndex(*prefix, " ")+1+indent)
	} else if strings.HasSuffix(*prefix, "-") {
		*prefix += " "
	}
	if comment := config.getComment(); comment != "" {
		for _, line := range strings.Split(comment, "\n") {
			if len(line) > 0 {
				line = " " + line
			}
			fmt.Fprintf(w, *prefix+"#%s\n", line)
			*prefix = strings.Repeat(" ", len(*prefix))
		}
	}
	condition := config.getCondition()
	if condition != "" {
		fmt.Fprintf(w, *prefix+"{{- %s }}\n", condition)
		*prefix = strings.Repeat(" ", len(*prefix))
	}
	writeValue()
	*prefix = strings.Repeat(" ", len(*prefix))
	if condition != "" {
		fmt.Fprintln(w, *prefix+"{{- end }}")
	}
}

// configMeta provides the shared metadata (comments & conditions) for all config types
type configMeta struct {
	comment   string
	condition string
}

func (meta configMeta) getComment() string             { return meta.comment }
func (meta configMeta) getCondition() string           { return meta.condition }
func (meta *configMeta) setComment(comment string)     { meta.comment = comment }
func (meta *configMeta) setCondition(condition string) { meta.condition = condition }

// ConfigScalar represents a scalar value inside a list or object
type ConfigScalar struct {
	configMeta
	value string
}

// NewConfigScalar creates a simple scalar node without comment or condition
func NewConfigScalar(value string) *ConfigScalar {
	return &ConfigScalar{value: value}
}

// NewConfigScalar creates a simple scalar node without comment or condition
func NewConfigScalarWithComment(value string, comment string) *ConfigScalar {
	return &ConfigScalar{configMeta{comment: comment}, value}
}

func (scalar ConfigScalar) write(w io.Writer, prefix string) {
	fmt.Fprintln(w, prefix+" "+scalar.value)
}

// ConfigList represents an ordered list of unnamed config nodes
type ConfigList struct {
	configMeta
	values []ConfigType
}

// add one or more config values at the end of the list
func (list *ConfigList) add(values ...ConfigType) {
	list.values = append(list.values, values...)
}

func (list ConfigList) write(w io.Writer, prefix string) {
	for _, value := range list.values {
		writeConditionAndComment(w, value, &prefix, 0, func() { value.write(w, prefix+"-") })
	}
}

type namedValue struct {
	name  string
	value ConfigType
}

// ConfigObject represents an ordered lst of named config values
type ConfigObject struct {
	configMeta
	values []namedValue
}

// add a singled named config value at the end of the list
func (object *ConfigObject) add(name string, value ConfigType) {
	object.values = append(object.values, namedValue{name: name, value: value})
}

func (object ConfigObject) write(w io.Writer, prefix string) {
	for _, namedValue := range object.values {
		name, value := namedValue.name, namedValue.value
		writeConditionAndComment(w, value, &prefix, 2, func() { value.write(w, prefix+name+":") })
	}
}

// WriteConfig writes templatized YAML config to Writer
func (object ConfigObject) WriteConfig(w io.Writer) error {
	fmt.Fprintln(w, "---")
	prefix := ""
	writeConditionAndComment(w, &object, &prefix, 0, func() { object.write(w, prefix) })
	return nil
}

//NewKubeConfig sets up generic Kube config structure with minimal metadata
func NewKubeConfig(kind string, name string) *ConfigObject {
	obj := &ConfigObject{}
	obj.add("apiVersion", NewConfigScalar("v1"))
	obj.add("kind", NewConfigScalar(kind))

	meta := ConfigObject{}
	meta.add("name", NewConfigScalar(name))
	obj.add("metadata", &meta)

	return obj
}
