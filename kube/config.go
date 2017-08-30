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

// ConfigScalar represents a scalar value inside a list or object
type ConfigScalar struct {
	value     string
	comment   string
	condition string
}

func (scalar ConfigScalar) getComment() string             { return scalar.comment }
func (scalar ConfigScalar) getCondition() string           { return scalar.condition }
func (scalar *ConfigScalar) setComment(comment string)     { scalar.comment = comment }
func (scalar *ConfigScalar) setCondition(condition string) { scalar.condition = condition }

func (scalar ConfigScalar) write(w io.Writer, prefix string) {
	fmt.Fprintln(w, prefix+" "+scalar.value)
}

// ConfigList represents an ordered list of unnamed config nodes
type ConfigList struct {
	values    []ConfigType
	comment   string
	condition string
}

func (list ConfigList) getComment() string             { return list.comment }
func (list ConfigList) getCondition() string           { return list.condition }
func (list *ConfigList) setComment(comment string)     { list.comment = comment }
func (list *ConfigList) setCondition(condition string) { list.condition = condition }

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
	values    []namedValue
	comment   string
	condition string
}

func (object ConfigObject) getComment() string             { return object.comment }
func (object ConfigObject) getCondition() string           { return object.condition }
func (object *ConfigObject) setComment(comment string)     { object.comment = comment }
func (object *ConfigObject) setCondition(condition string) { object.condition = condition }

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

// NewConfigScalar creates a simple scalar node without comment or condition
func NewConfigScalar(value string) *ConfigScalar {
	return &ConfigScalar{value: value}
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
