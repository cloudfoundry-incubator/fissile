package helm

/*

Package helm implements a generic config file writer, emitting templatized YAML
documents. The documents are built up in memory from Scalar, List, and Object
nodes, which all implement the Node interface.

Each node can have an associated comment, and be wrapped by a template
condition. For example:

  obj.Add("Answer", NewScalar("42", Comment("A comment"), Condition(".Values.enabled")))

will generate:

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


Examples:

* Throw an error if the the configuration can not possibly work

    list.Add(NewScalar("{{ fail \"Cannot proceed\" }}", Condition(".count <= 0")))

* Use a condition to generate multiple list elements

    tcp := NewObject(Condition("range $key, $value := .Values.tcp"))
    tcp.Add("name", NewScalar("\"{{ $key }}-tcp\""))
    tcp.Add("containerPort", NewScalar("$key"))
    tcp.Add("protocol", NewScalar("TCP"))

    ports := NewList(Comment("List of TCP ports"))
    ports.Add(tcp)
    root.Add("ports", ports)

  Generates this document:

    # List of TCP ports
    ports:
    {{- range $key, $value := .Values.tcp }}
    - name: "{{ $key }}-tcp"
      containerPort: $key
      protocol: TCP
    {{- end }}

*/

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// sharedFields provides the shared metadata (comments & conditions) for all
// node types.
type sharedFields struct {
	comment   string
	condition string
}

// NodeModifier functions can be used to set values of shared fields in a node.
type NodeModifier func(*sharedFields)

// Comment returns a modifier function to set the comment of a node.
func Comment(comment string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.comment = comment }
}

// Condition returns a modifier function to set the condition of a node.
func Condition(condition string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.condition = condition }
}

// Set applies NodeModifier functions to the embedded sharedFields struct.
func (shared *sharedFields) Set(modifiers ...NodeModifier) {
	for _, modifier := range modifiers {
		modifier(shared)
	}
}

// Comment is a shared getter function for all node types.
func (shared sharedFields) Comment() string {
	return shared.comment
}

// Condition is a shared getter function for all node types.
func (shared sharedFields) Condition() string {
	return shared.condition
}

// Node is the interface implemented by all config node types.
type Node interface {
	// Every node will embed a sharedFields struct and inherit these methods:
	Comment() string
	Condition() string
	Set(...NodeModifier)

	// The write() method is specific to each node type. prefix will be the
	// indented label, either "name:" or "-", depending on whether the node
	// is part of an object or a list.
	write(enc *Encoder, prefix string)
}

// Scalar represents a scalar value inside a list or object.
type Scalar struct {
	sharedFields
	value string
}

// NewScalar creates a scalar node and initializes shared fields.
func NewScalar(value string, modifiers ...NodeModifier) *Scalar {
	scalar := &Scalar{value: value}
	scalar.Set(modifiers...)
	return scalar
}

func (scalar Scalar) write(enc *Encoder, prefix string) {
	if strings.ContainsAny(scalar.value, "\n") {
		// Scalars including newlines will be written using the "literal" YAML format.
		fmt.Fprintln(enc.writer, prefix+" |-")
		// Calculate proper indentation for data lines.
		if strings.HasSuffix(prefix, ":") {
			prefix = strings.Repeat(" ", strings.LastIndex(prefix, " ")+1+enc.indent)
		} else {
			prefix = strings.Repeat(" ", len(prefix)+1)
		}
		for _, line := range strings.Split(scalar.value, "\n") {
			fmt.Fprintln(enc.writer, prefix+line)
		}
	} else {
		fmt.Fprintln(enc.writer, prefix+" "+scalar.value)
	}
}

// List represents an ordered list of unnamed nodes.
type List struct {
	sharedFields
	nodes []Node
}

// NewList creates an empty list node and initializes shared fields.
func NewList(modifiers ...NodeModifier) *List {
	list := &List{}
	list.Set(modifiers...)
	return list
}

// Add one or more nodes at the end of the list.
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

// Object represents an ordered lst of named nodes.
type Object struct {
	sharedFields
	nodes []namedNode
}

// NewObject creates an empty object node and initializes shared fields.
func NewObject(modifiers ...NodeModifier) *Object {
	object := &Object{}
	object.Set(modifiers...)
	return object
}

// Add a singled named node at the end of the list.
func (object *Object) Add(name string, node Node) {
	object.nodes = append(object.nodes, namedNode{name: name, node: node})
}

// Sort all nodes of the object by name.
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

// Encoder writes the config data to an output stream.
type Encoder struct {
	writer io.Writer

	indent int
	wrap   int

	emptyLines     bool
	pendingNewline bool
}

// EmptyLines turns generation of additional empty lines on or off. In general
// each node that has a comment or a condition will be separated by additional
// empty lines from the rest of the document. The leading empty line will be
// omitted for the first element of a list or object, and the trailing empty
// line will be omitted for the last element. The default value is false.
func EmptyLines(emptyLines bool) func(*Encoder) {
	return func(enc *Encoder) {
		enc.emptyLines = emptyLines
	}
}

// Indent sets the indentation amount per nesting level for the YAML encoding.
// The default value is 2.
func Indent(indent int) func(*Encoder) {
	return func(enc *Encoder) {
		if indent < 2 {
			indent = 2
		}
		enc.indent = indent
	}
}

// Wrap sets the maximum line length for comments. This number includes the
// columns needed for indentation, so comments on deeper nested nodes have more
// tightly wrapped comments than outer level nodes. Wrapping applies only to
// comments and not to conditions or scalar values. The default value is 80.
func Wrap(wrap int) func(*Encoder) {
	return func(enc *Encoder) {
		enc.wrap = wrap
	}
}

// Set applies Encoder modifier functions to set option values.
func (enc *Encoder) Set(modifiers ...func(*Encoder)) {
	for _, modifier := range modifiers {
		modifier(enc)
	}
}

// NewEncoder returns an Encoder object wrapping the output stream and encoding options
func NewEncoder(writer io.Writer, modifiers ...func(*Encoder)) *Encoder {
	enc := &Encoder{
		// Wrap io.Writer in a bufio.Writer so that we can check for
		// errors at the very end by calling bufio.Writer.Flush()
		writer:     bufio.NewWriter(writer),
		emptyLines: false,
		indent:     2,
		wrap:       80,
	}
	enc.Set(modifiers...)
	return enc
}

// Encode writes the config object to the stream.
func (enc *Encoder) Encode(object *Object) error {
	enc.pendingNewline = false
	fmt.Fprintln(enc.writer, "---")
	prefix := ""
	enc.writeNode(object, &prefix, 0, "")
	return enc.writer.(*bufio.Writer).Flush()
}

// useOnce returns the current value of the prefix parameter but replaces it
// with a string of spaces of the same length as the original prefix fo
// subsequent use. This is done because for nested list elements the "- "
// prefixes are being stacked, and should only be used in front of the first
// element:
//
//   [ [ [ 1, 2 ] ] ]   --->   " - - - 1\n    - 2\n"
//
func useOnce(prefix *string) string {
	result := *prefix
	*prefix = strings.Repeat(" ", len(*prefix))
	return result
}

// writeComment writes out the comment block for a node. Newline characters in
// the comment mark the beginning of a new paragraph. Each paragraph is
// word-wrapped to fit within enc.wrap columns (except that each line will
// include at least a single word, even if it exceeds the wrapping column).
func (enc *Encoder) writeComment(prefix *string, comment string) {
	for _, line := range strings.Split(comment, "\n") {
		fmt.Fprintf(enc.writer, "%s#", useOnce(prefix))
		if len(line) > 0 {
			written := 0
			for _, word := range strings.Fields(line) {
				if written > 0 && len(*prefix)+1+written+1+len(word) > enc.wrap {
					fmt.Fprintf(enc.writer, "\n%s#", useOnce(prefix))
					written = 0
				}
				fmt.Fprint(enc.writer, " "+word)
				written += 1 + len(word)
			}
		}
		fmt.Fprint(enc.writer, "\n")
	}
}

// writeNode is called for each element of a list or object node. It will print
// the comment and condition for the element and then call node.write() to tell
// the node to encode itself.
//
// indent specifies the number of additional columns to indent the value.
//
// value contains either the "name:" for object elements, or (a possibly
// stacked list of) " -" list element markers. Named prefix will be printed
// immediately while list prefixes are accumulated until the first value is
// printed that is not a list itself.
//
func (enc *Encoder) writeNode(node Node, prefix *string, indent int, value string) {
	leadingNewline := enc.emptyLines
	if enc.pendingNewline {
		fmt.Fprint(enc.writer, "\n")
		enc.pendingNewline = false
		leadingNewline = false
	}
	if strings.HasSuffix(*prefix, ":") {
		fmt.Fprintln(enc.writer, *prefix)
		*prefix = strings.Repeat(" ", strings.LastIndex(*prefix, " ")+1+indent)
		leadingNewline = false
	} else if strings.HasSuffix(*prefix, "-") {
		*prefix += " "
		leadingNewline = false
	} else if *prefix == "" && indent == 0 {
		leadingNewline = false
	}
	comment := node.Comment()
	condition := node.Condition()
	if leadingNewline && (comment != "" || condition != "") {
		fmt.Fprint(enc.writer, "\n")
	}
	if comment != "" {
		enc.writeComment(prefix, comment)
	}
	if condition != "" {
		fmt.Fprintf(enc.writer, "%s{{- %s }}\n", useOnce(prefix), condition)
	}
	node.write(enc, useOnce(prefix)+value)
	if condition != "" {
		fmt.Fprintln(enc.writer, *prefix+"{{- end }}")
	}
	if comment != "" || condition != "" {
		enc.pendingNewline = enc.emptyLines
	}
}
