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

// NodeModifier functions can be used to set values of shared fields in a Node
type NodeModifier func(*sharedFields)

// Comment returns a modifier function to set the comment of a Node
func Comment(comment string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.comment = comment }
}

// Condition returns a modifier function to set the condition of a Node
func Condition(condition string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.condition = condition }
}

// Apply NodeModifier functions to set shared fields
func (shared *sharedFields) Apply(modifiers ...NodeModifier) {
	for _, modifier := range modifiers {
		modifier(shared)
	}
}

func (shared sharedFields) Comment() string   { return shared.comment }
func (shared sharedFields) Condition() string { return shared.condition }

// Node is the interface implemented by all config node types
type Node interface {
	// Every node will embed a sharedFields struct and inherit these methods:
	Apply(...NodeModifier)
	Comment() string
	Condition() string
	// The write() method is specific to each Node type
	write(enc Encoder, prefix string)
}

// Scalar represents a scalar value inside a list or object
type Scalar struct {
	sharedFields
	value string
}

// NewScalar creates a scalar node and initializes shared fields
func NewScalar(value string, modifiers ...NodeModifier) *Scalar {
	scalar := &Scalar{value: value}
	scalar.Apply(modifiers...)
	return scalar
}

func (scalar Scalar) write(enc Encoder, prefix string) {
	if strings.ContainsAny(scalar.value, "\n") {
		fmt.Fprintln(enc.w, prefix+" |-")
		if strings.HasSuffix(prefix, ":") {
			prefix = strings.Repeat(" ", strings.LastIndex(prefix, " ")+1+enc.indent)
		} else {
			prefix = strings.Repeat(" ", len(prefix)+1)
		}
		for _, line := range strings.Split(scalar.value, "\n") {
			fmt.Fprintln(enc.w, prefix+line)
		}
	} else {
		fmt.Fprintln(enc.w, prefix+" "+scalar.value)
	}
}

// List represents an ordered list of unnamed nodes
type List struct {
	sharedFields
	nodes []Node
}

// NewList creates an empty list node and initializes shared fields
func NewList(modifiers ...NodeModifier) *List {
	list := &List{}
	list.Apply(modifiers...)
	return list
}

// Add one or more nodes at the end of the list
func (list *List) Add(nodes ...Node) {
	list.nodes = append(list.nodes, nodes...)
}

func (list List) write(enc Encoder, prefix string) {
	for _, node := range list.nodes {
		enc.writeNode(node, &prefix, 0, strings.Repeat(" ", enc.indent-2)+"-")
	}
}

// Objects store nodes in a list of name/node pairs instead of using a map so that
// the nodes will be encoded in the same order in which they were added to the object.
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
func NewObject(modifiers ...NodeModifier) *Object {
	object := &Object{}
	object.Apply(modifiers...)
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

// Encoder writes the config data to an output stream
type Encoder struct {
	w      io.Writer
	indent int
	wrap   int
}

// Indent sets the indentation amount for the YAML encoding
func Indent(indent int) func(*Encoder) {
	return func(enc *Encoder) {
		if indent < 2 {
			panic("helm.Encoder indent must be at least 2")
		}
		enc.indent = indent
	}
}

// Wrap sets the maximum line length for comments
func Wrap(wrap int) func(*Encoder) {
	return func(enc *Encoder) {
		enc.wrap = wrap
	}
}

// Apply Encoder modifier functions to set options
func (enc *Encoder) Apply(modifiers ...func(*Encoder)) {
	for _, modifier := range modifiers {
		modifier(enc)
	}
}

// NewEncoder returns an Encoder object wrapping the output stream and encoding options
func NewEncoder(w io.Writer, modifiers ...func(*Encoder)) *Encoder {
	enc := &Encoder{w: w, indent: 2, wrap: 80}
	enc.Apply(modifiers...)
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

func (enc Encoder) writeComment(prefix *string, comment string) {
	for _, line := range strings.Split(comment, "\n") {
		fmt.Fprintf(enc.w, "%s#", useOnce(prefix))
		if len(line) > 0 {
			written := 0
			for _, field := range strings.Fields(line) {
				if written > 0 && len(*prefix)+1+written+1+len(field) > enc.wrap {
					fmt.Fprintf(enc.w, "\n%s#", useOnce(prefix))
					written = 0
				}
				fmt.Fprint(enc.w, " "+field)
				written += 1 + len(field)
			}
			line = " " + line
		}
		fmt.Fprint(enc.w, "\n")
	}
}

func (enc Encoder) writeNode(node Node, prefix *string, indent int, value string) {
	if strings.HasSuffix(*prefix, ":") {
		fmt.Fprintln(enc.w, *prefix)
		*prefix = strings.Repeat(" ", strings.LastIndex(*prefix, " ")+1+indent)
	} else if strings.HasSuffix(*prefix, "-") {
		*prefix += " "
	}
	if comment := node.Comment(); comment != "" {
		enc.writeComment(prefix, comment)
	}
	condition := node.Condition()
	if condition != "" {
		fmt.Fprintf(enc.w, "%s{{- %s }}\n", useOnce(prefix), condition)
	}
	node.write(enc, useOnce(prefix)+value)
	if condition != "" {
		fmt.Fprintln(enc.w, *prefix+"{{- end }}")
	}
}
