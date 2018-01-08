/*

Package helm implements a generic config file writer, emitting templatized YAML
documents. The documents are built up in memory from Scalar, List, and Mapping
nodes, which all implement the Node interface.

Each node can have an associated comment, and be wrapped by a template block
(any template action that encloses text and terminates with an {{end}}). For
example:

  map.Add("Answer", NewNode("42", Comment("A comment"), Block("if .Values.enabled")))

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

    list.Add(NewNode(`{{ fail "Cannot proceed" }}`, Block("if le (int .count) 0")))

* Use a block action to generate multiple list elements

    tcp := NewMapping().Set(Block("range $key, $value := .Values.tcp"))
    tcp.Add("name", NewNode("\"{{ $key }}-tcp\""))
    tcp.Add("containerPort", NewNode("$key"))
    tcp.Add("protocol", NewNode("TCP"))

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
	"reflect"
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
func (shared *sharedFields) SetValue(interface{}) {
	panic("SetValue can only be called on helm.Scalar nodes")
}

// Values returns all the elements of a list node.
func (shared *sharedFields) Values() []Node {
	panic("Values can only be called on helm.List nodes")
}

// Get returns a named node from a mapping node.
func (shared *sharedFields) Get(...string) Node {
	panic("Get can only be called on helm.Mapping nodes")
}

// Node is the interface implemented by all config node types.
type Node interface {
	// Every node will embed a sharedFields struct and inherit these methods:
	Block() string
	Comment() string
	Set(...NodeModifier)

	// Scalar node methods:
	SetValue(interface{})
	String() string

	// List node methods:
	Values() []Node

	// Mapping node methods:
	Get(...string) Node

	// The write() method implements the part of Encoder.writeNode() that
	// needs to access the fields specific to each node type. prefix will be
	// the indented label, either "name:" or "-", depending on whether the
	// node is part of a mapping or a list.
	write(enc *Encoder, prefix string)
}

// NewNode creates a new node (scalar of various types, list, map)
// depending on the (runtime type of the) specified value.
func NewNode(value interface{}, modifiers ...NodeModifier) Node {
	var node Node
	if value == nil {
		node = &Scalar{value: "~"}
	} else if _, ok := value.(Node); ok {
		node = value.(Node)
	} else {
		switch reflect.TypeOf(value).Kind() {
		case reflect.Map:
			valueOf := reflect.ValueOf(value)
			mapping := NewMapping()
			for _, key := range valueOf.MapKeys() {
				mapping.Add(fmt.Sprintf("%s", key.Interface()), valueOf.MapIndex(key).Interface())
			}
			node = mapping.Sort()
		case reflect.Slice:
			valueOf := reflect.ValueOf(value)
			list := NewList()
			for i := 0; i < valueOf.Len(); i++ {
				list.Add(valueOf.Index(i).Interface())
			}
			node = list
		case reflect.Bool:
			node = &Scalar{value: strconv.FormatBool(value.(bool))}
		case reflect.Int:
			node = &Scalar{value: strconv.FormatInt(int64(value.(int)), 10)}
		case reflect.Float64:
			node = &Scalar{value: strconv.FormatFloat(value.(float64), 'G', -1, 64)}
		case reflect.String:
			str := value.(string)
			// don't quote values that are templates from start to end
			if !(strings.HasPrefix(str, "{{") && strings.HasSuffix(str, "}}")) {
				str = strconv.Quote(str)
			}
			node = &Scalar{value: str}
		default:
			panic(fmt.Sprintf("Unexpected type: %s", reflect.TypeOf(value).Kind()))
		}
	}
	node.Set(modifiers...)
	return node
}

// Scalar represents a scalar value inside a list or mapping.
type Scalar struct {
	sharedFields
	value string
}

// SetValue updates the value of a scalar node.
func (scalar *Scalar) SetValue(value interface{}) {
	node := NewNode(value)
	scalar.value = node.(*Scalar).value
}

// String returns the string value of a scalar node. This value will *not*
// include the surrounding quotes used for literal YAML strings.
func (scalar *Scalar) String() string {
	if strings.HasPrefix(scalar.value, `"`) && strings.HasSuffix(scalar.value, `"`) {
		unquoted, _ := strconv.Unquote(scalar.value)
		return unquoted
	}
	return scalar.value
}

func (scalar Scalar) write(enc *Encoder, prefix string) {
	fmt.Fprintln(enc, prefix+" "+strings.Replace(scalar.value, "\n", "\\n", -1))
}

// List represents an ordered list of unnamed nodes.
type List struct {
	sharedFields
	nodes []Node
}

// NewList creates a list node initialized with a slice of values.
// The list and element nodes will not have comments nor block actions.
func NewList(values ...interface{}) *List {
	list := &List{}
	list.Add(values...)
	return list
}

// Add one or more nodes at the end of the list.
func (list *List) Add(values ...interface{}) {
	for _, value := range values {
		list.nodes = append(list.nodes, NewNode(value))
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
	emptyLines := enc.useEmptyLines(prefix, list.nodes)
	if len(list.nodes) == 0 {
		var leadingSpace = ""
		if prefix != "" {
			leadingSpace = " "
		}
		fmt.Fprintln(enc, prefix+leadingSpace+"[]")
	}
	for _, node := range list.nodes {
		enc.writeNode(node, &prefix, strings.Repeat(" ", enc.indent-2)+"-", emptyLines)
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

// Mapping represents an ordered list of named nodes.
type Mapping struct {
	sharedFields
	nodes []namedNode
}

// NewMapping creates a new mapping node initialized with a slice of
// keys and values, interleaved. The keys are stored in the
// even-numbered indices and must be strings. Values are odd-numbered.
// The mapping and element nodes will not have comments nor block
// actions.
func NewMapping(values ...interface{}) *Mapping {
	mapping := &Mapping{}
	for i := 0; i < len(values); i += 2 {
		var value interface{}
		if i+1 < len(values) {
			value = values[i+1]
		}
		mapping.Add(values[i].(string), value)
	}
	return mapping
}

// Add a single named node at the end of the list, replacing any existing
// entries of the same name.
func (mapping *Mapping) Add(name string, value interface{}, modifiers ...NodeModifier) {
	for i, node := range mapping.nodes {
		if node.name == name {
			mapping.nodes = append(mapping.nodes[:i], mapping.nodes[i+1:]...)
			// There can be at most one node with this name
			break
		}
	}
	mapping.nodes = append(mapping.nodes, namedNode{name: name, node: NewNode(value, modifiers...)})
}

// Get returns the named node, or nil if the name cannot be found. Name may be
// a "/" separated "path" of nested names, which will be treated as a string of
// Get() calls, but with the advantage of not crashing if any intermediate node
// does not exist.
func (mapping *Mapping) Get(names ...string) Node {
	if len(names) > 0 {
		for _, namedNode := range mapping.nodes {
			if names[0] == namedNode.name {
				if len(names) > 1 {
					return namedNode.node.Get(names[1:]...)
				}
				return namedNode.node
			}
		}
	}
	return nil
}

// Names returns the ordered list of node names in the mapping.
func (mapping *Mapping) Names() []string {
	names := make([]string, 0, len(mapping.nodes))
	for _, namedNode := range mapping.nodes {
		names = append(names, namedNode.name)
	}
	return names
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
	var nodes []Node
	for _, namedNode := range mapping.nodes {
		nodes = append(nodes, namedNode.node)
	}
	emptyLines := enc.useEmptyLines(prefix, nodes)
	for _, namedNode := range mapping.nodes {
		enc.writeNode(namedNode.node, &prefix, namedNode.name+":", emptyLines)
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
// The default value is 2. This is also the minimum allowed.
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

// NewEncoder returns an Encoder holding the output stream and encoding options.
func NewEncoder(writer io.Writer, modifiers ...func(*Encoder)) *Encoder {
	enc := &Encoder{
		writer:     writer,
		err:        nil,
		emptyLines: true,
		indent:     2,
		wrap:       80,
	}
	enc.Set(modifiers...)
	return enc
}

// Encode writes the config mapping held by the node to the stream.
func (enc *Encoder) Encode(node Node) error {
	enc.pendingNewline = false
	fmt.Fprintln(enc, "---")
	prefix := ""
	enc.writeNode(node, &prefix, "", enc.emptyLines)
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

// useEmptyLines determines if the elements of a list or mapping should use
// empty lines. It uses the encoder setting, but disables it for nodes where
// only a single element has a comment or block action. It is not disabled for
// the top-level document, so that there can be an empty line between the
// document comment and the comment of the only element of a document root.
func (enc *Encoder) useEmptyLines(prefix string, nodes []Node) bool {
	emptyLines := enc.emptyLines
	if prefix != "" {
		specialElements := 0
		for _, node := range nodes {
			if node.Block() != "" || node.Comment() != "" {
				specialElements++
			}
		}
		if specialElements <= 1 {
			emptyLines = false
		}
	}
	return emptyLines
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
// the comment mark the beginning of a new paragraph (but trailing newlines are
// ignored).
//
// Each paragraph is word-wrapped to fit within enc.wrap columns (except that
// each line will include at least a single word, even if it exceeds the
// wrapping column).
//
// Paragraphs starting with "* " or "- " are treated as bullet points, so
// wrapped lines will be indented by 2 spaces.
func (enc *Encoder) writeComment(prefix *string, comment string) {
	for _, line := range strings.Split(strings.TrimRight(comment, "\n"), "\n") {
		fmt.Fprintf(enc, "%s#", useOnce(prefix))
		if len(line) > 0 {
			written := 0
			bullet := strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "- ")
			for _, word := range strings.Fields(line) {
				if written > 0 && len(*prefix)+1+written+1+len(word) > enc.wrap {
					fmt.Fprintf(enc, "\n%s#", useOnce(prefix))
					written = 0
					if bullet {
						fmt.Fprint(enc, "  ")
						written = 2
					}
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
//
// EmptyLines is set by the caller (via Encode.useEmptyLines) to enc.emptyLines,
// except when there is only a single element with a comment or block action, in
// which case it will be `false`.
func (enc *Encoder) writeNode(node Node, prefix *string, label string, emptyLines bool) {
	leadingNewline := emptyLines
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
		enc.pendingNewline = emptyLines
	}
}
