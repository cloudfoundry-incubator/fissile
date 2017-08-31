package helm

import (
	"fmt"
	"io"
	"strings"
)

// Node is the interface implemented by all config node types
type Node interface {
	getComment() string
	getCondition() string
	setComment(string)
	setCondition(string)
	write(enc Encoder, prefix string)
}

func useOnce(prefix *string) string {
	result := *prefix
	*prefix = strings.Repeat(" ", len(*prefix))
	return result
}

func writeNode(enc Encoder, node Node, prefix *string, indent int, value string) {
	if strings.HasSuffix(*prefix, ":") {
		fmt.Fprintln(enc.w, *prefix)
		*prefix = strings.Repeat(" ", strings.LastIndex(*prefix, " ")+1+indent)
	} else if strings.HasSuffix(*prefix, "-") {
		*prefix += " "
	}
	if comment := node.getComment(); comment != "" {
		for _, line := range strings.Split(comment, "\n") {
			if len(line) > 0 {
				line = " " + line
			}
			fmt.Fprintf(enc.w, "%s#%s\n", useOnce(prefix), line)
		}
	}
	condition := node.getCondition()
	if condition != "" {
		fmt.Fprintf(enc.w, "%s{{- %s }}\n", useOnce(prefix), condition)
	}
	node.write(enc, useOnce(prefix)+value)
	if condition != "" {
		fmt.Fprintln(enc.w, *prefix+"{{- end }}")
	}
}

// sharedFields provides the shared metadata (comments & conditions) for all Node types
type sharedFields struct {
	comment   string
	condition string
}

func (shared sharedFields) getComment() string             { return shared.comment }
func (shared sharedFields) getCondition() string           { return shared.condition }
func (shared *sharedFields) setComment(comment string)     { shared.comment = comment }
func (shared *sharedFields) setCondition(condition string) { shared.condition = condition }

// Scalar represents a scalar value inside a list or object
type Scalar struct {
	sharedFields
	value string
}

// NewScalar creates a simple scalar node without comment or condition
func NewScalar(value string) *Scalar {
	return &Scalar{value: value}
}

// NewScalarWithComment creates a simple scalar node with comment
func NewScalarWithComment(value string, comment string) *Scalar {
	return &Scalar{sharedFields{comment: comment}, value}
}

func (scalar Scalar) write(enc Encoder, prefix string) {
	fmt.Fprintln(enc.w, prefix+" "+scalar.value)
}

// List represents an ordered list of unnamed nodes
type List struct {
	sharedFields
	nodes []Node
}

// Add one or more nodes at the end of the list
func (list *List) Add(nodes ...Node) {
	list.nodes = append(list.nodes, nodes...)
}

func (list List) write(enc Encoder, prefix string) {
	for _, node := range list.nodes {
		writeNode(enc, node, &prefix, 0, "-")
	}
}

type namedNode struct {
	name string
	node Node
}

// Object represents an ordered lst of named nodes
type Object struct {
	sharedFields
	nodes []namedNode
}

// Add a singled named node at the end of the list
func (object *Object) Add(name string, node Node) {
	object.nodes = append(object.nodes, namedNode{name: name, node: node})
}

func (object Object) write(enc Encoder, prefix string) {
	for _, namedNode := range object.nodes {
		name, node := namedNode.name, namedNode.node
		writeNode(enc, node, &prefix, enc.indent, name+":")
	}
}

//NewKubeConfig sets up generic a Kube config structure with minimal metadata
func NewKubeConfig(kind string, name string) *Object {
	obj := &Object{}
	obj.Add("apiVersion", NewScalar("v1"))
	obj.Add("kind", NewScalar(kind))

	meta := Object{}
	meta.Add("name", NewScalar(name))
	obj.Add("metadata", &meta)

	return obj
}

// Encoder writes the config data to an output stream
type Encoder struct {
	w      io.Writer
	indent int
}

// NewEncoder returns an Encoder object wrapping the output stream and encoding options
func NewEncoder(w io.Writer) *Encoder {
	enc := &Encoder{w: w, indent: 2}
	return enc
}

// Encode writes the config object to the stream
func (enc Encoder) Encode(obj *Object) error {
	fmt.Fprintln(enc.w, "---")
	prefix := ""
	writeNode(enc, obj, &prefix, 0, "")
	return nil
}
