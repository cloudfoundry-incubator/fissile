/*

Package helm implements a generic config file writer, emitting templatized YAML
documents. The documents are built up in memory from Scalar, List, and Mapping
nodes, which all implement the Node interface.

Each node can have an associated comment, and be wrapped by a template block
(any template action that encloses text and terminates with an {{end}}). For
example:

  obj.Add("Answer", NewScalar("42", Comment("A comment"), Block("if .Values.enabled")))

will generate:

  # A comment
  {{- if .Values.enabled }}
  Answer: 42
  {{- end }}


Scalar values are emitted as-is, that means the value needs to include quotes if
it needs to be quoted to be valid YAML.  Literal values are also not
line-wrapped because that might break template expressions.

The only exception are literal values including newlines. These values will be
emitted with newlines intact, and subsequent lines indented to match the
document structure. This means lines containing literal newlines characters
should not be quoted (and conversely, newlines in quoted strings need to be
escaped, e.g. "\"Multiple\nLines\"".

An Encoder object holds an io.Writer target as well as additional encoding
options, like the max line length for comments, or the YAML indentation level:

  NewEncoder(os.Stdout, Indent(4), Wrap(80)).Encode(documentRoot)

Tricks:

* Throw an error if the the configuration cannot possibly work

    list.Add(NewScalar(`{{ fail "Cannot proceed" }}`, Block("if .count <= 0")))

* Use a block action to generate multiple list elements

    tcp := NewMapping(Block("range $key, $value := .Values.tcp"))
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
package helm

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// sharedFields provides the shared metadata (comments & block actions) for all
// node types.
type sharedFields struct {
	block   string
	comment string
}

// NodeModifier functions can be used to set values of shared fields in a node.
type NodeModifier func(*sharedFields)

// Block returns a modifier function to set the block action of a node.
func Block(block string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.block = block }
}

// Comment returns a modifier function to set the comment of a node.
func Comment(comment string) func(*sharedFields) {
	return func(shared *sharedFields) { shared.comment = comment }
}

// Set applies NodeModifier functions to the embedded sharedFields struct.
func (shared *sharedFields) Set(modifiers ...NodeModifier) {
	for _, modifier := range modifiers {
		modifier(shared)
	}
}

// Block is a shared getter function for all node types.
func (shared sharedFields) Block() string {
	return shared.block
}

// Comment is a shared getter function for all node types.
func (shared sharedFields) Comment() string {
	return shared.comment
}

// SetValue updates the value of a scalar node.
func (shared *sharedFields) SetValue(string) {
	panic("SetValue can only be called on helm.Scalar nodes")
}

// Value returns the value of a scalar node.
func (shared *sharedFields) Value() string {
	panic("Value can only be called on helm.Scalar nodes")
}

// Values returns all the elements of a list node.
func (shared *sharedFields) Values() []Node {
	panic("Values can only be called on helm.List nodes")
}

// Get returns a named node from a mapping node.
func (shared *sharedFields) Get(string) Node {
	panic("Get can only be called on helm.Mapping nodes")
}

// Node is the interface implemented by all config node types.
type Node interface {
	// Every node will embed a sharedFields struct and inherit these methods:
	Block() string
	Comment() string
	Set(...NodeModifier)

	// Scalar node methods:
	SetValue(string)
	Value() string

	// List node methods:
	Values() []Node

	// Mapping node methods:
	Get(string) Node

	// The write() method implements the part of Encoder.writeNode() that
	// needs to access the fields specific to each node type. prefix will be
	// the indented label, either "name:" or "-", depending on whether the
	// node is part of a mapping or a list.
	write(enc *Encoder, prefix string)
}

// Scalar represents a scalar value inside a list or mapping.
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

// SetValue updates the value of a scalar node.
func (scalar *Scalar) SetValue(value string) {
	scalar.value = value
}

func (scalar *Scalar) String() string {
	buffer := &bytes.Buffer{}
	NewEncoder(buffer).Encode(scalar)
	return buffer.String()
}

// Value returns the value of a scalar node.
func (scalar *Scalar) Value() string {
	return scalar.value
}

func (scalar Scalar) write(enc *Encoder, prefix string) {
	if !strings.ContainsRune(scalar.value, '\n') {
		fmt.Fprintln(enc, prefix+" "+scalar.value)
		return
	}

	// Scalars including newlines will be written using the "literal" YAML format.
	fmt.Fprintln(enc, prefix+" |-")
	// Calculate proper indentation for data lines.
	if strings.HasSuffix(prefix, ":") {
		prefix = strings.Repeat(" ", strings.LastIndex(prefix, " ")+1+enc.indent)
	} else {
		prefix = strings.Repeat(" ", len(prefix)+1)
	}
	for _, line := range strings.Split(scalar.value, "\n") {
		fmt.Fprintln(enc, prefix+line)
	}
}

// List represents an ordered list of unnamed nodes.
type List struct {
	sharedFields
	nodes []Node
}

// NewEmptyList creates an empty list node and initializes shared fields.
func NewEmptyList(modifiers ...NodeModifier) *List {
	list := &List{}
	list.Set(modifiers...)
	return list
}

// NewList creates a list node initialized with nodes.
// The list node will not have a comment or block action.
func NewList(nodes ...Node) *List {
	list := &List{}
	list.Add(nodes...)
	return list
}

// NewStrList creates a list node initialized with string scalars.
// The list and scalar nodes will not have comments or block actions.
func NewStrList(values ...string) *List {
	list := &List{}
	for _, value := range values {
		list.nodes = append(list.nodes, NewScalar(value))
	}
	return list
}

// Add one or more nodes at the end of the list.
func (list *List) Add(nodes ...Node) {
	for _, node := range nodes {
		if node != nil {
			list.nodes = append(list.nodes, node)
		}
	}
}

func (list *List) String() string {
	buffer := &bytes.Buffer{}
	NewEncoder(buffer).Encode(list)
	return buffer.String()
}

// Values returns a slice of all the elements of the list.
func (list *List) Values() []Node {
	return list.nodes
}

func (list List) write(enc *Encoder, prefix string) {
	for _, node := range list.nodes {
		enc.writeNode(node, &prefix, strings.Repeat(" ", enc.indent-2)+"-")
	}
}

// Mappings store nodes in a list of name/node pairs instead of using a map so
// that the nodes will be encoded in the same order in which they were added to
// the mapping. Names must be alphanumeric; they cannot contain whitespace or
// special characters.
type namedNode struct {
	name string
	node Node
}

// Mapping represents an ordered lst of named nodes.
type Mapping struct {
	sharedFields
	nodes []namedNode
}

// NewEmptyMapping creates an empty mapping node and initializes shared fields.
func NewEmptyMapping(modifiers ...NodeModifier) *Mapping {
	mapping := &Mapping{}
	mapping.Set(modifiers...)
	return mapping
}

// NewMapping creates a single node mapping node and initializes shared fields.
func NewMapping(name string, node Node, modifiers ...NodeModifier) *Mapping {
	mapping := &Mapping{}
	mapping.Set(modifiers...)
	mapping.Add(name, node)
	return mapping
}

// NewIntMapping creates a new mapping containing a single named integer scalar.
// The scalar node will not have a comment or block action.
func NewIntMapping(name string, value int, modifiers ...NodeModifier) *Mapping {
	mapping := &Mapping{}
	mapping.Set(modifiers...)
	mapping.AddInt(name, value)
	return mapping
}

// NewStrMapping creates a new mapping node initialzed with string scalars.
// The mapping and scalar nodes will not have comments or block actions.
func NewStrMapping(values ...string) *Mapping {
	mapping := &Mapping{}
	for i := 0; i < len(values); i += 2 {
		value := "~"
		if i+1 < len(values) {
			value = values[i+1]
		}
		mapping.AddStr(values[i], value)
	}
	return mapping
}

// Add a singled named node at the end of the list.
func (mapping *Mapping) Add(name string, node Node) {
	if node != nil {
		mapping.nodes = append(mapping.nodes, namedNode{name: name, node: node})
	}
}

// AddBool adds a named boolean if the value is true.
func (mapping *Mapping) AddBool(name string, value bool, modifiers ...NodeModifier) {
	if value {
		mapping.Add(name, NewScalar("true", modifiers...))
	}
}

// AddInt adds a named integer if the value is not 0.
func (mapping *Mapping) AddInt(name string, value int, modifiers ...NodeModifier) {
	if value != 0 {
		mapping.Add(name, NewScalar(strconv.Itoa(value), modifiers...))
	}
}

// AddStr adds a named string if the value is not the empty string.
func (mapping *Mapping) AddStr(name string, value string, modifiers ...NodeModifier) {
	if value != "" {
		mapping.Add(name, NewScalar(value, modifiers...))
	}
}

// Get returns the named node, or nil if the name cannot be found.
func (mapping *Mapping) Get(name string) Node {
	for _, namedNode := range mapping.nodes {
		if name == namedNode.name {
			return namedNode.node
		}
	}
	return nil
}

// Keys returns the the ordered list of node names in the mapping.
func (mapping *Mapping) Keys() []string {
	var keys []string
	for _, namedNode := range mapping.nodes {
		keys = append(keys, namedNode.name)
	}
	return keys
}

// Merge appends all named nodes from another mapping.
func (mapping *Mapping) Merge(merge *Mapping) {
	mapping.nodes = append(mapping.nodes, merge.nodes...)
}

// Sort all nodes of the mapping by name.
func (mapping *Mapping) Sort() *Mapping {
	sort.Slice(mapping.nodes[:], func(i, j int) bool {
		return mapping.nodes[i].name < mapping.nodes[j].name
	})
	return mapping
}

func (mapping *Mapping) String() string {
	buffer := &bytes.Buffer{}
	NewEncoder(buffer).Encode(mapping)
	return buffer.String()
}

func (mapping Mapping) write(enc *Encoder, prefix string) {
	for _, namedNode := range mapping.nodes {
		enc.writeNode(namedNode.node, &prefix, namedNode.name+":")
	}
}

// Encoder writes the config data to an output stream.
type Encoder struct {
	writer io.Writer
	// err keeps track of any error being returned when writing to writer.
	err error

	// indent specifies the number of columns per YAML nesting level.
	indent int

	// wrap specifies the maximum line length for comments.
	wrap int

	// emptyLines specifies that values with comments/blocks should be
	// surrounded by empty lines for easier grouping.
	emptyLines bool

	// pendingNewline is an internal flag to only emit a single empty line
	// between elements that both require surrounding empty lines.
	pendingNewline bool
}

// EmptyLines turns generation of additional empty lines on or off. In general
// each node that has a comment or a block action will be separated by
// additional empty lines from the rest of the document. The leading empty line
// will be omitted for the first element of a list or mapping, and the trailing
// empty line will be omitted for the last element. The default value is false.
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
// columns needed for indentation, so comments on more deeply nested nodes have
// more tightly wrapped comments than outer level nodes. Wrapping applies only
// to comments and not to block actions or scalar values. The default value is 80.
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

// NewEncoder returns an Encoder mapping holding the output stream and encoding options.
func NewEncoder(writer io.Writer, modifiers ...func(*Encoder)) *Encoder {
	enc := &Encoder{
		writer:     writer,
		err:        nil,
		emptyLines: false,
		indent:     2,
		wrap:       80,
	}
	enc.Set(modifiers...)
	return enc
}

// Encode writes the config mapping to the stream.
func (enc *Encoder) Encode(node Node) error {
	enc.pendingNewline = false
	fmt.Fprintln(enc, "---")
	prefix := ""
	enc.writeNode(node, &prefix, "")
	return enc.err
}

// Write implements the io.Writer interface. It just forwards to the embedded
// writer until an error occurs. This allows for error checking just once at the
// end of Encode().
func (enc *Encoder) Write(buffer []byte) (int, error) {
	if enc.err == nil {
		var written int
		written, enc.err = enc.writer.Write(buffer)
		return written, enc.err
	}
	return 0, enc.err
}

// useOnce returns the current value of the prefix parameter but replaces it
// with a string of spaces of the same length as the original prefix for
// subsequent use. This is done because for nested list elements the "- "
// prefixes are being stacked, and should only be used in front of the first
// element of the innermost list:
//
//   [ [ [ 1, 2 ] ] ]   --->   " - - - 1\n    - 2\n"
//
func useOnce(prefix *string) string {
	result := *prefix
	*prefix = strings.Repeat(" ", len(*prefix))
	return result
}

// writeComment writes out the comment lines for a node. Newline characters in
// the comment mark the beginning of a new paragraph. Each paragraph is
// word-wrapped to fit within enc.wrap columns (except that each line will
// include at least a single word, even if it exceeds the wrapping column).
func (enc *Encoder) writeComment(prefix *string, comment string) {
	for _, line := range strings.Split(comment, "\n") {
		fmt.Fprintf(enc, "%s#", useOnce(prefix))
		if len(line) > 0 {
			written := 0
			for _, word := range strings.Fields(line) {
				if written > 0 && len(*prefix)+1+written+1+len(word) > enc.wrap {
					fmt.Fprintf(enc, "\n%s#", useOnce(prefix))
					written = 0
				}
				fmt.Fprint(enc, " "+word)
				written += 1 + len(word)
			}
		}
		fmt.Fprint(enc, "\n")
	}
}

// writeNode is called for each element of a container (list or mapping
// node). It will print the comment and block action for the element and then
// call node.write() to tell the node to encode itself.
//
// prefix includes indentation and the label for the container of this element
// (either "Name:" or a string of one or more "- "). It will only be printed in
// front of the first element of the container and then be replaced with a
// string of spaces for indentation of all remaining elements. The label is not
// printed by the container itself because the list labels "- " stack up on a
// single line.
//
// label contains either the "Name:" for mapping elements, or "-" for list
// elements. The special value "" is used for the document root, which doesn't
// have a label.
func (enc *Encoder) writeNode(node Node, prefix *string, label string) {
	leadingNewline := enc.emptyLines
	if enc.pendingNewline {
		fmt.Fprint(enc, "\n")
		enc.pendingNewline = false
		leadingNewline = false
	}
	indent := 0
	if strings.HasSuffix(label, ":") {
		indent = enc.indent
	}
	if strings.HasSuffix(*prefix, ":") {
		fmt.Fprintln(enc, *prefix)
		*prefix = strings.Repeat(" ", strings.LastIndex(*prefix, " ")+1+indent)
		leadingNewline = false
	} else if strings.HasSuffix(*prefix, "-") {
		*prefix += " "
		leadingNewline = false
	} else if label == "" {
		leadingNewline = false
	}
	comment := node.Comment()
	block := node.Block()
	if leadingNewline && (comment != "" || block != "") {
		fmt.Fprint(enc, "\n")
	}
	if comment != "" {
		enc.writeComment(prefix, comment)
	}
	if block != "" {
		fmt.Fprintf(enc, "%s{{- %s }}\n", useOnce(prefix), block)
	}
	node.write(enc, useOnce(prefix)+label)
	if block != "" {
		fmt.Fprintln(enc, *prefix+"{{- end }}")
	}
	if comment != "" || block != "" {
		enc.pendingNewline = enc.emptyLines
	}
}
