package helm

/*

Package helm implements a generic config file writer, emitting templatized YAML
documents. The documents are built up in memory from Scalar, List, and Object
nodes, which all implement the Node interface.

Each node can have an associated comment, and be wrapped by a template
condition:

  obj.Add("Answer", NewScalar("42", Comment("A comment"), Condition(".Values.enabled")))

  # A comment
  {{- .Values.enabled }}
  Answer: 42
  {{- end }}


Scalar values are emitted as-is, that means the value needs to include quotes if
it needs to be quoted to be valid YAML.  Liternal values are also not
line-wrapped because that might break template expressions.

The only exception are literal values including newlines. These values will be
emitted with newlines intact, and subsequent lines indented to match the
document structure. This means lines containing literal newlines characters
should not be quoted (and conversely, newlines in quoted strings need to be
escaped, e.g. "\"Multiple\nLines\"".

An Encoder objects holds the io.Writer target as well as additional encoding
options, like the max line length for comments, or the YAML indentation level:

  NewEncoder(os.Stdout, Indent(4), Wrap(80)).Encode(documentRoot)


*/

import (
	"bufio"
	"fmt"
	"io"
	"sort"
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

// Set applies NodeModifier functions to the embedded sharedFields struct
func (shared *sharedFields) Set(modifiers ...NodeModifier) {
	for _, modifier := range modifiers {
		modifier(shared)
	}
}

// Comment is a shared getter function for all Node types
func (shared sharedFields) Comment() string {
	return shared.comment
}

// Condition is a shared getter function for all Node types
func (shared sharedFields) Condition() string {
	return shared.condition
}

// Node is the interface implemented by all config node types
type Node interface {
	// Every node will embed a sharedFields struct and inherit these methods:
	Comment() string
	Condition() string
	Set(...NodeModifier)

	// The write() method is specific to each Node type. prefix will be the
	// indented label, either "name:" or "-", depending on whether the node
	// is part of an object or a list
	write(enc *Encoder, prefix string)
}

// Scalar represents a scalar value inside a list or object
type Scalar struct {
	sharedFields
	value string
}

// NewScalar creates a scalar node and initializes shared fields
func NewScalar(value string, modifiers ...NodeModifier) *Scalar {
	scalar := &Scalar{value: value}
	scalar.Set(modifiers...)
	return scalar
}

func (scalar Scalar) write(enc *Encoder, prefix string) {
	if strings.ContainsAny(scalar.value, "\n") {
		// Scalars including newlines will be written using the "literal" YAML format
		fmt.Fprintln(enc.w, prefix+" |-")
		// Calculate proper indentation for all data lines
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
	list.Set(modifiers...)
	return list
}

// Add one or more nodes at the end of the list
func (list *List) Add(nodes ...Node) {
	list.nodes = append(list.nodes, nodes...)
}

func (list List) write(enc *Encoder, prefix string) {
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
	object.Set(modifiers...)
	return object
}

// Add a singled named node at the end of the list
func (object *Object) Add(name string, node Node) {
	object.nodes = append(object.nodes, namedNode{name: name, node: node})
}

// Sort all nodes of the object by name
func (object *Object) Sort() {
	sort.Slice(object.nodes[:], func(i, j int) bool {
		return object.nodes[i].name < object.nodes[j].name
	})
}

func (object Object) write(enc *Encoder, prefix string) {
	for _, namedNode := range object.nodes {
		enc.writeNode(namedNode.node, &prefix, enc.indent, namedNode.name+":")
	}
}

// Encoder writes the config data to an output stream
type Encoder struct {
	w io.Writer

	indent int
	wrap   int

	emptyLines     bool
	pendingNewline bool
}

// EmptyLines switches generation of additional empty lines on or off. In
// general each node that has a comment or a condition will be separate by
// additional empty lines from the rest of the document. The leading empty line
// will be omitted for the first element of a list of object, and the trailing
// empty line will be omitted for the last element.
func EmptyLines(emptyLines bool) func(*Encoder) {
	return func(enc *Encoder) {
		enc.emptyLines = emptyLines
	}
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

// Wrap sets the maximum line length for comments. This number includes the
// columns needed for indentation, so comments on deeper nodes have more tightly
// wrapped comments than outer nodes. Wrapping applies only to comments and not
// to conditions or scalar values.
func Wrap(wrap int) func(*Encoder) {
	return func(enc *Encoder) {
		enc.wrap = wrap
	}
}

// Set applies Encoder modifier functions to set options
func (enc *Encoder) Set(modifiers ...func(*Encoder)) {
	for _, modifier := range modifiers {
		modifier(enc)
	}
}

// NewEncoder returns an Encoder object wrapping the output stream and encoding options
func NewEncoder(w io.Writer, modifiers ...func(*Encoder)) *Encoder {
	enc := &Encoder{w: bufio.NewWriter(w), emptyLines: false, indent: 2, wrap: 80}
	enc.Set(modifiers...)
	return enc
}

// Encode writes the config object to the stream
func (enc *Encoder) Encode(obj *Object) error {
	enc.pendingNewline = false
	fmt.Fprintln(enc.w, "---")
	prefix := ""
	enc.writeNode(obj, &prefix, 0, "")
	return enc.w.(*bufio.Writer).Flush()
}

func useOnce(prefix *string) string {
	result := *prefix
	*prefix = strings.Repeat(" ", len(*prefix))
	return result
}

func (enc *Encoder) writeComment(prefix *string, comment string) {
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
		}
		fmt.Fprint(enc.w, "\n")
	}
}

func (enc *Encoder) writeNode(node Node, prefix *string, indent int, value string) {
	newline := enc.emptyLines
	if enc.pendingNewline {
		fmt.Fprint(enc.w, "\n")
		enc.pendingNewline = false
		newline = false
	}
	if strings.HasSuffix(*prefix, ":") {
		fmt.Fprintln(enc.w, *prefix)
		*prefix = strings.Repeat(" ", strings.LastIndex(*prefix, " ")+1+indent)
		newline = false
	} else if strings.HasSuffix(*prefix, "-") {
		*prefix += " "
		newline = false
	} else if *prefix == "" && indent == 0 {
		newline = false
	}
	comment := node.Comment()
	condition := node.Condition()
	if newline && (comment != "" || condition != "") {
		fmt.Fprint(enc.w, "\n")
	}
	if comment != "" {
		enc.writeComment(prefix, comment)
	}
	if condition != "" {
		fmt.Fprintf(enc.w, "%s{{- %s }}\n", useOnce(prefix), condition)
	}
	node.write(enc, useOnce(prefix)+value)
	if condition != "" {
		fmt.Fprintln(enc.w, *prefix+"{{- end }}")
	}
	if comment != "" || condition != "" {
		enc.pendingNewline = enc.emptyLines
	}
}
