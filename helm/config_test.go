package helm

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

// annotate by recursively adding comments or block actions to each node
func annotate(node Node, comment bool, index int) int {
	index++
	if comment {
		node.Set(Comment(fmt.Sprintf("comment %d", index)))
	} else {
		node.Set(Block(fmt.Sprintf("if block %d", index)))
	}
	switch node.(type) {
	case *List:
		for _, node := range node.(*List).nodes {
			index = annotate(node, comment, index)
		}
	case *Mapping:
		for _, namedNode := range node.(*Mapping).nodes {
			index = annotate(namedNode.node, comment, index)
		}
	}
	return index
}
func addComments(node Node) { annotate(node, true, 0) }
func addBlocks(node Node)   { annotate(node, false, 0) }

func equal(t *testing.T, config Node, expect string, modifiers ...func(*Encoder)) {
	buffer := &bytes.Buffer{}
	enc := NewEncoder(buffer, EmptyLines(false))
	enc.Set(modifiers...)
	assert.NoError(t, enc.Encode(config))
	assert.Equal(t, expect, buffer.String())
}

func TestHelmScalar(t *testing.T) {
	scalar := NewNode("Foo\nBar")
	equal(t, scalar, fmt.Sprintf("---\n %q\n", scalar))

	root := NewMapping("Scalar", scalar)

	equal(t, root, `---
Scalar: "Foo\nBar"
`)
	equal(t, root, fmt.Sprintf("%s", root))

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Scalar: "Foo\nBar"
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
Scalar: "Foo\nBar"
{{- end }}
{{- end }}
`)
}

func TestHelmList(t *testing.T) {
	list := NewList(1, "2")
	list.Add(3.1415, "{{ no quotes }}", nil, false, "")

	root := NewMapping("List", list)

	equal(t, root, `---
List:
- 1
- "2"
- 3.1415
- {{ no quotes }}
- ~
- false
- ""
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
List:
# comment 3
- 1
# comment 4
- "2"
# comment 5
- 3.1415
# comment 6
- {{ no quotes }}
# comment 7
- ~
# comment 8
- false
# comment 9
- ""
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
List:
# comment 3
{{- if block 3 }}
- 1
{{- end }}
# comment 4
{{- if block 4 }}
- "2"
{{- end }}
# comment 5
{{- if block 5 }}
- 3.1415
{{- end }}
# comment 6
{{- if block 6 }}
- {{ no quotes }}
{{- end }}
# comment 7
{{- if block 7 }}
- ~
{{- end }}
# comment 8
{{- if block 8 }}
- false
{{- end }}
# comment 9
{{- if block 9 }}
- ""
{{- end }}
{{- end }}
{{- end }}
`)
	values := list.Values()
	assert.Equal(t, 7, len(values))
	assert.Equal(t, "3.1415", values[2].String())

	equal(t, NewList(), `---
[]
`)
	equal(t, NewList(NewList()), `---
- []
`)
	equal(t, NewList(NewList(), "1"), `---
- []
- "1"
`)
}

func TestHelmMapping(t *testing.T) {
	mapping := NewMapping("foo", 1)
	mapping.Add("bar", "too")
	mapping.Add("bar", "two")

	root := NewMapping("Mapping", mapping)

	equal(t, root, `---
Mapping:
  foo: 1
  bar: "two"
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Mapping:
  # comment 3
  foo: 1
  # comment 4
  bar: "two"
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
Mapping:
  # comment 3
  {{- if block 3 }}
  foo: 1
  {{- end }}
  # comment 4
  {{- if block 4 }}
  bar: "two"
  {{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmListOfList(t *testing.T) {
	list1 := NewList()
	list1.Add(1)
	list1.Add(2)

	list2 := NewList()
	list2.Add(list1)
	list2.Add("x")
	list2.Add("y")

	list3 := NewList()
	list3.Add(list2)
	list3.Add("foo")
	list3.Add("bar")

	root := NewMapping("List", list3)

	equal(t, root, `---
List:
- - - 1
    - 2
  - "x"
  - "y"
- "foo"
- "bar"
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
List:
# comment 3
- # comment 4
  - # comment 5
    - 1
    # comment 6
    - 2
  # comment 7
  - "x"
  # comment 8
  - "y"
# comment 9
- "foo"
# comment 10
- "bar"
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
List:
# comment 3
{{- if block 3 }}
- # comment 4
  {{- if block 4 }}
  - # comment 5
    {{- if block 5 }}
    - 1
    {{- end }}
    # comment 6
    {{- if block 6 }}
    - 2
    {{- end }}
  {{- end }}
  # comment 7
  {{- if block 7 }}
  - "x"
  {{- end }}
  # comment 8
  {{- if block 8 }}
  - "y"
  {{- end }}
{{- end }}
# comment 9
{{- if block 9 }}
- "foo"
{{- end }}
# comment 10
{{- if block 10 }}
- "bar"
{{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmMappingOfMapping(t *testing.T) {
	mapping1 := NewMapping()
	mapping1.Add("One", 1)
	mapping1.Add("Two", 2)

	mapping2 := NewMapping()
	mapping2.Add("OneTwo", mapping1)
	mapping2.Add("X", "x")
	mapping2.Add("Y", "y")

	mapping3 := NewMapping()
	mapping3.Add("XY", mapping2)
	mapping3.Add("Foo", "foo")
	mapping3.Add("Bar", "bar")

	root := NewMapping("Mapping", mapping3)

	equal(t, root, `---
Mapping:
  XY:
    OneTwo:
      One: 1
      Two: 2
    X: "x"
    Y: "y"
  Foo: "foo"
  Bar: "bar"
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Mapping:
  # comment 3
  XY:
    # comment 4
    OneTwo:
      # comment 5
      One: 1
      # comment 6
      Two: 2
    # comment 7
    X: "x"
    # comment 8
    Y: "y"
  # comment 9
  Foo: "foo"
  # comment 10
  Bar: "bar"
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
Mapping:
  # comment 3
  {{- if block 3 }}
  XY:
    # comment 4
    {{- if block 4 }}
    OneTwo:
      # comment 5
      {{- if block 5 }}
      One: 1
      {{- end }}
      # comment 6
      {{- if block 6 }}
      Two: 2
      {{- end }}
    {{- end }}
    # comment 7
    {{- if block 7 }}
    X: "x"
    {{- end }}
    # comment 8
    {{- if block 8 }}
    Y: "y"
    {{- end }}
  {{- end }}
  # comment 9
  {{- if block 9 }}
  Foo: "foo"
  {{- end }}
  # comment 10
  {{- if block 10 }}
  Bar: "bar"
  {{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmMappingOfList(t *testing.T) {
	list := NewList()
	list.Add(1)
	list.Add(2)
	list.Add(3)

	root := NewMapping("Mapping", NewMapping("List", list))

	equal(t, root, `---
Mapping:
  List:
  - 1
  - 2
  - 3
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Mapping:
  # comment 3
  List:
  # comment 4
  - 1
  # comment 5
  - 2
  # comment 6
  - 3
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
Mapping:
  # comment 3
  {{- if block 3 }}
  List:
  # comment 4
  {{- if block 4 }}
  - 1
  {{- end }}
  # comment 5
  {{- if block 5 }}
  - 2
  {{- end }}
  # comment 6
  {{- if block 6 }}
  - 3
  {{- end }}
  {{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmListOfMapping(t *testing.T) {
	mapping := NewMapping()
	mapping.Add("Foo", "foo")
	mapping.Add("Bar", "bar")
	mapping.Add("Baz", "baz")

	list := NewList()
	list.Add(mapping)

	root := NewMapping("Mapping", list)

	equal(t, root, `---
Mapping:
- Foo: "foo"
  Bar: "bar"
  Baz: "baz"
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Mapping:
# comment 3
- # comment 4
  Foo: "foo"
  # comment 5
  Bar: "bar"
  # comment 6
  Baz: "baz"
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
Mapping:
# comment 3
{{- if block 3 }}
- # comment 4
  {{- if block 4 }}
  Foo: "foo"
  {{- end }}
  # comment 5
  {{- if block 5 }}
  Bar: "bar"
  {{- end }}
  # comment 6
  {{- if block 6 }}
  Baz: "baz"
  {{- end }}
{{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmMultiLineComment(t *testing.T) {
	root := NewMapping("Scalar", NewNode(42, Comment("Many\n\nlines")))

	equal(t, root, `---
# Many
#
# lines
Scalar: 42
`)

	// list of list
	list1 := NewList()
	list1.Add(NewNode(42, Comment("Many\n\nlines")))

	list2 := NewList()
	list2.Add(list1)
	list2.Add("foo")

	root = NewMapping("List", list2)

	equal(t, root, `---
List:
- # Many
  #
  # lines
  - 42
- "foo"
`)
}

func TestHelmWrapLongComments(t *testing.T) {
	root := NewMapping()
	root.Add("Bullet", NewNode(nil, Comment("* "+strings.Repeat("abc 12345 ", 5)+"\n\n- "+strings.Repeat("abcd 12345 ", 5))))

	mapping := NewMapping()
	word := "1"
	for i := len(word) + 1; i < 7; i++ {
		word += strconv.Itoa(i)
		root.Add(fmt.Sprintf("Key%d", i), NewNode(nil, Comment(strings.Repeat(word+" ", 10))))
		if i < 5 {
			mapping.Add(fmt.Sprintf("Key%d", i), NewNode(nil, Comment(strings.Repeat(word+" ", 5))))
		}
	}

	mapping.Add("Very", NewNode("Long", Comment(" "+strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.Add("Very", NewNode("Long", Comment(strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.Add("Nested", mapping, Comment("None\n One\n  Two\n   Three"))

	expect := `---
# * abc 12345 abc 12345
#   abc 12345 abc 12345
#   abc 12345
#
# - abcd 12345 abcd
#   12345 abcd 12345
#   abcd 12345 abcd
#   12345
Bullet: ~
# 12 12 12 12 12 12 12
# 12 12 12
Key2: ~
# 123 123 123 123 123
# 123 123 123 123 123
Key3: ~
# 1234 1234 1234 1234
# 1234 1234 1234 1234
# 1234 1234
Key4: ~
# 12345 12345 12345
# 12345 12345 12345
# 12345 12345 12345
# 12345
Key5: ~
# 123456 123456 123456
# 123456 123456 123456
# 123456 123456 123456
# 123456
Key6: ~
# xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
# xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Very: "Long"
# None
#  One
#   Two
#    Three
Nested:
          # 12 12 12 12
          # 12
          Key2: ~
          # 123 123 123
          # 123 123
          Key3: ~
          # 1234 1234
          # 1234 1234
          # 1234
          Key4: ~
          #  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
          #  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
          Very: "Long"
`
	equal(t, root, expect, Indent(10), Wrap(24))
}

func TestHelmIndent(t *testing.T) {
	mapping1 := NewMapping()
	mapping1.Add("Foo", NewNode("Bar", Comment("Baz")))

	list1 := NewList()
	list1.Add(mapping1)
	list1.Add(1)

	list2 := NewList()
	list2.Add("abc")
	list2.Add("xyz")

	list1.Add(list2)
	list1.Add(2)
	list1.Add(3)

	mapping2 := NewMapping()
	mapping2.Add("List", list1)

	mapping3 := NewMapping()
	mapping3.Add("Foo", 1)
	mapping3.Add("Bar", 2)

	mapping2.Add("Meta", mapping3)

	root := NewMapping()
	root.Add("Mapping", mapping2)

	expect := `---
Mapping:
    List:
      - # Baz
        Foo: "Bar"
      - 1
      -   - "abc"
          - "xyz"
      - 2
      - 3
    Meta:
        Foo: 1
        Bar: 2
`
	equal(t, root, expect, Indent(4))
}

func TestHelmEncoderModifier(t *testing.T) {
	mapping := NewMapping()
	mapping.Add("foo", 1)
	mapping.Add("bar", 2)
	mapping.Add("baz", 3)

	root := NewMapping("Mapping", mapping)

	expect := `---
Mapping:
  foo: 1
  bar: 2
  baz: 3
---
Mapping:
    foo: 1
    bar: 2
    baz: 3
`

	buffer := &bytes.Buffer{}
	enc := NewEncoder(buffer, Indent(0))
	enc.Encode(root)
	enc.Set(Indent(4))
	enc.Encode(root)
	assert.Equal(t, expect, buffer.String())
}

func TestHelmMultiLineScalar(t *testing.T) {
	root := NewMapping("Scalar", "foo\nbar\nbaz")
	list := NewList("one\ntwo\nthree")
	root.Add("List", list)

	expect := `---
Scalar: "foo\nbar\nbaz"
List:
  - "one\ntwo\nthree"
`
	equal(t, root, expect, Indent(4))
}

func TestHelmEmptyMapping(t *testing.T) {
	root := NewMapping()
	mapping := NewMapping()
	root.Add("Mapping", mapping)
	expect := `---
Mapping: {}
`
	equal(t, root, expect)
}

func TestHelmEmptyLines(t *testing.T) {
	list := NewList()
	list.Add(NewNode(1, Comment("Some comment")))
	list.Add(NewNode(2, Comment("")))
	list.Add(NewNode(3, Comment("Another comment")))
	list.Add(4)

	mapping := NewMapping()
	mapping.Set(Comment("Mapping comment"))
	mapping.Add("List", list)
	mapping.Add("One", NewNode(1, Comment("First post")))
	mapping.Add("Two", 2)
	mapping.Add("Three", NewNode(3, Block("if .Values.set")))

	root := NewMapping("Mapping", mapping)
	root.Set(Comment("Top level comment"))

	expect := `---
# Top level comment

# Mapping comment
Mapping:
  List:
  # Some comment
  - 1

  - 2

  # Another comment
  - 3

  - 4

  # First post
  One: 1

  Two: 2

  {{- if .Values.set }}
  Three: 3
  {{- end }}
`
	equal(t, root, expect, EmptyLines(true))

	expect = `---
# comment 1
{{- if block 1 }}

# comment 2
{{- if block 2 }}
Mapping:
  # comment 3
  {{- if block 3 }}
  List:
  # comment 4
  {{- if block 4 }}
  - 1
  {{- end }}

  # comment 5
  {{- if block 5 }}
  - 2
  {{- end }}

  # comment 6
  {{- if block 6 }}
  - 3
  {{- end }}

  # comment 7
  {{- if block 7 }}
  - 4
  {{- end }}
  {{- end }}

  # comment 8
  {{- if block 8 }}
  One: 1
  {{- end }}

  # comment 9
  {{- if block 9 }}
  Two: 2
  {{- end }}

  # comment 10
  {{- if block 10 }}
  Three: 3
  {{- end }}
{{- end }}
{{- end }}
`
	addComments(root)
	addBlocks(root)
	equal(t, root, expect, EmptyLines(true))

	list = NewList()
	list.Add(1)
	list.Add(NewNode(2, Comment("A comment")))
	list.Add(3)

	root = NewMapping("List", list)

	expect = `---
List:
- 1
# A comment
- 2
- 3
`
	equal(t, root, expect, EmptyLines(true))

	list.Add(NewNode(4, Comment("Another comment")))
	root = NewMapping("List", list)

	expect = `---
List:
- 1

# A comment
- 2

- 3

# Another comment
- 4
`
	equal(t, root, expect, EmptyLines(true))
}

func TestHelmMappingSort(t *testing.T) {
	mapping := NewMapping()
	mapping.Add("foo", 1)
	mapping.Add("bar", 2)
	mapping.Add("baz", 3)

	root := NewMapping("Mapping", mapping.Sort())

	equal(t, root, `---
Mapping:
  bar: 2
  baz: 3
  foo: 1
`)
	names := mapping.Names()
	assert.Equal(t, 3, len(names))
	assert.Equal(t, "foo", names[2])
	assert.Equal(t, "1", mapping.Get(names[2]).String())
}

func TestHelmError(t *testing.T) {
	root := NewMapping("Foo", 1)

	buffer := &bytes.Buffer{}
	enc := NewEncoder(buffer)
	enc.err = errors.New("monkey wrench")

	assert.NotNil(t, enc.Encode(root))
	assert.Equal(t, "", buffer.String())
}

func TestHelmMerge(t *testing.T) {
	root := NewMapping("One", 1)
	root.Merge(NewMapping("Foo", 2))

	equal(t, root, `---
One: 1
Foo: 2
`)
}

func TestHelmAddValues(t *testing.T) {
	root := NewMapping("One", 1, "Two", 2, "Three")
	root.Add("List", NewList("X", "Y"))
	root.Add("Int", 42, Block("if condition"))
	root.Add("Nodes", NewList(NewNode("foo"), NewNode("bar")))
	equal(t, root, `---
One: 1
Two: 2
Three: ~
List:
- "X"
- "Y"
{{- if condition }}
Int: 42
{{- end }}
Nodes:
- "foo"
- "bar"
`)
}

func TestHelmAddNilNodes(t *testing.T) {
	root := NewMapping()
	root.Add("Foo", "X")
	root.Add("Bar", nil)
	root.Add("Baz", "Y")
	root.Add("List", NewList(1, nil, 2))
	equal(t, root, `---
Foo: "X"
Bar: ~
Baz: "Y"
List:
- 1
- ~
- 2
`)
}

func TestHelmGetNode(t *testing.T) {
	root := NewMapping("Foo", 1, "Bar", 2)
	assert.Nil(t, root.Get("Baz"))

	bar := root.Get("Bar")
	assert.NotNil(t, bar)

	if bar != nil {
		assert.Equal(t, bar.String(), "2")
		bar.SetValue(3)
	}

	root.Add("Baz", NewMapping("xyzzy", "plugh"))
	assert.Equal(t, "plugh", root.Get("Baz", "xyzzy").String())

	equal(t, root, `---
Foo: 1
Bar: 3
Baz:
  xyzzy: "plugh"
`)
}

func TestHelmEncodeList(t *testing.T) {
	root := NewList("One", "Two")
	equal(t, root, `---
- "One"
- "Two"
`)
	equal(t, root, fmt.Sprintf("%s", root))
}

func hasPanicked(test func()) (panicked bool) {
	panicked = false
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	test()
	return
}

func TestHelmPanic(t *testing.T) {
	assert.False(t, hasPanicked(func() { NewNode("foo").SetValue("new") }))
	assert.True(t, hasPanicked(func() { NewList().SetValue("new") }))
	assert.True(t, hasPanicked(func() { NewMapping().SetValue("new") }))

	assert.True(t, hasPanicked(func() { NewNode("foo").Values() }))
	assert.False(t, hasPanicked(func() { NewList().Values() }))
	assert.True(t, hasPanicked(func() { NewMapping().Values() }))

	assert.True(t, hasPanicked(func() { NewNode("foo").Get("key") }))
	assert.True(t, hasPanicked(func() { NewList().Get("key") }))
	assert.False(t, hasPanicked(func() { NewMapping().Get("key") }))

	assert.True(t, hasPanicked(func() { NewNode(func() { return }) }))
}

func TestHelmReflect(t *testing.T) {
	expect := `---
Bar: "xyzzy plugh"
Baz: ~
Bool: true
Float: 1.23
Foo: 123
List:
- 1
- 2
- 3
Nil: ~
`
	var tree interface{}
	assert.NoError(t, yaml.Unmarshal([]byte(expect), &tree))
	actual := NewNode(tree)
	equal(t, actual, expect)

	// Encode a reflected struct and use it as documentation
	buffer := &bytes.Buffer{}
	enc := NewEncoder(buffer, Separator(false))
	enc.Encode(actual, "  ")

	root := NewMapping("Scalar", NewNode(42, Comment("Example:\n"+buffer.String())))
	equal(t, root, `---
# Example:
#   Bar: "xyzzy plugh"
#   Baz: ~
#   Bool: true
#   Float: 1.23
#   Foo: 123
#   List:
#   - 1
#   - 2
#   - 3
#   Nil: ~
Scalar: 42
`)
}
