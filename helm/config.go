package helm

import (
	"fmt"
	"io"
	"strings"
)

// sharedFields provides the shared metadata (comments & conditions) for all Node types
type sharedFields struct {
	comment   string
	condition string
}

// Comment returns a modifier function to set the comment of a Node
func Comment(comment string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.comment = comment }
}

// Condition returns a modifier function to set the condition of a Node
func Condition(condition string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.condition = condition }
}

// apply sharedFields modifier functions that can be passed to the constructors of the nodes
func (shared *sharedFields) apply(modifiers []func(*sharedFields)) {
	for _, modifier := range modifiers {
		modifier(shared)
	}
}

func (shared sharedFields) getComment() string             { return shared.comment }
func (shared sharedFields) getCondition() string           { return shared.condition }
func (shared *sharedFields) setComment(comment string)     { shared.comment = comment }
func (shared *sharedFields) setCondition(condition string) { shared.condition = condition }

// Node is the interface implemented by all config node types
type Node interface {
	// Every node will embed a sharedFields struct and inherit these methods:
	apply([]func(*sharedFields))
	getComment() string
	getCondition() string
	setComment(string)
	setCondition(string)
	// The write() method is specific to each Node type
	write(enc Encoder, prefix string)
}

// Scalar represents a scalar value inside a list or object
type Scalar struct {
	sharedFields
	value string
}

// NewScalar creates a scalar node and initializes shared fields
func NewScalar(value string, modifiers ...func(*sharedFields)) *Scalar {
	scalar := &Scalar{value: value}
	scalar.apply(modifiers)
	return scalar
}

func (scalar Scalar) write(enc Encoder, prefix string) {
	fmt.Fprintln(enc.w, prefix+" "+scalar.value)
}

// List represents an ordered list of unnamed nodes
type List struct {
	sharedFields
	nodes []Node
}

// NewList creates an empty list node and initializes shared fields
func NewList(modifiers ...func(*sharedFields)) *List {
	list := &List{}
	list.apply(modifiers)
	return list
}

// Add one or more nodes at the end of the list
func (list *List) Add(nodes ...Node) {
	list.nodes = append(list.nodes, nodes...)
}

func (list List) write(enc Encoder, prefix string) {
	for _, node := range list.nodes {
		enc.writeNode(node, &prefix, 0, "-")
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

// NewObject creates an empty object node and initializes shared fields
func NewObject(modifiers ...func(*sharedFields)) *Object {
	object := &Object{}
	object.apply(modifiers)
	return object
}

// Add a singled named node at the end of the list
func (object *Object) Add(name string, node Node) {
	object.nodes = append(object.nodes, namedNode{name: name, node: node})
}

func (object Object) write(enc Encoder, prefix string) {
	for _, namedNode := range object.nodes {
		enc.writeNode(namedNode.node, &prefix, enc.indent, namedNode.name+":")
	}
}

// NewKubeConfig sets up generic a Kube config structure with minimal metadata (move this to "kube" package?)
func NewKubeConfig(kind string, name string, modifiers ...func(*sharedFields)) *Object {
	object := NewObject(modifiers...)
	object.Add("apiVersion", NewScalar("v1"))
	object.Add("kind", NewScalar(kind))

	meta := NewObject()
	meta.Add("name", NewScalar(name))
	object.Add("metadata", meta)

	return object
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
	enc.writeNode(obj, &prefix, 0, "")
	return nil
}

func useOnce(prefix *string) string {
	result := *prefix
	*prefix = strings.Repeat(" ", len(*prefix))
	return result
}

func (enc Encoder) writeNode(node Node, prefix *string, indent int, value string) {
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
