package helm

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Nil(t, NewEncoder(buffer, modifiers...).Encode(config))
	assert.Equal(t, expect, buffer.String())
}

func TestHelmScalar(t *testing.T) {
	scalar := NewScalar("42")
	equal(t, scalar, fmt.Sprintf("%s", scalar))

	root := NewNodeMapping("Scalar", scalar)

	equal(t, root, `---
Scalar: 42
`)
	equal(t, root, fmt.Sprintf("%s", root))

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Scalar: 42
`)

	addBlocks(root)
	equal(t, root, `---
# comment 1
{{- if block 1 }}
# comment 2
{{- if block 2 }}
Scalar: 42
{{- end }}
{{- end }}
`)
}

func TestHelmList(t *testing.T) {
	list := NewList("1", "2")
	list.AddNode(NewScalar("3"))

	root := NewNodeMapping("List", list)

	equal(t, root, `---
List:
- 1
- 2
- 3
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
List:
# comment 3
- 1
# comment 4
- 2
# comment 5
- 3
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
- 2
{{- end }}
# comment 5
{{- if block 5 }}
- 3
{{- end }}
{{- end }}
{{- end }}
`)
	values := list.Values()
	assert.Equal(t, 3, len(values))
	assert.Equal(t, "3", values[2].Value())
}

func TestHelmMapping(t *testing.T) {
	mapping := NewIntMapping("foo", 1)
	mapping.Add("bar", "two")

	root := NewNodeMapping("Mapping", mapping)

	equal(t, root, `---
Mapping:
  foo: 1
  bar: two
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Mapping:
  # comment 3
  foo: 1
  # comment 4
  bar: two
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
  bar: two
  {{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmListOfList(t *testing.T) {
	list1 := NewEmptyList()
	list1.AddNode(NewScalar("1"))
	list1.AddNode(NewScalar("2"))

	list2 := NewEmptyList()
	list2.AddNode(list1)
	list2.AddNode(NewScalar("x"))
	list2.AddNode(NewScalar("y"))

	list3 := NewEmptyList()
	list3.AddNode(list2)
	list3.AddNode(NewScalar("foo"))
	list3.AddNode(NewScalar("bar"))

	root := NewNodeMapping("List", list3)

	equal(t, root, `---
List:
- - - 1
    - 2
  - x
  - y
- foo
- bar
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
  - x
  # comment 8
  - y
# comment 9
- foo
# comment 10
- bar
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
  - x
  {{- end }}
  # comment 8
  {{- if block 8 }}
  - y
  {{- end }}
{{- end }}
# comment 9
{{- if block 9 }}
- foo
{{- end }}
# comment 10
{{- if block 10 }}
- bar
{{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmMappingOfMapping(t *testing.T) {
	mapping1 := NewEmptyMapping()
	mapping1.AddNode("One", NewScalar("1"))
	mapping1.AddNode("Two", NewScalar("2"))

	mapping2 := NewEmptyMapping()
	mapping2.AddNode("OneTwo", mapping1)
	mapping2.AddNode("X", NewScalar("x"))
	mapping2.AddNode("Y", NewScalar("y"))

	mapping3 := NewEmptyMapping()
	mapping3.AddNode("XY", mapping2)
	mapping3.AddNode("Foo", NewScalar("foo"))
	mapping3.AddNode("Bar", NewScalar("bar"))

	root := NewNodeMapping("Mapping", mapping3)

	equal(t, root, `---
Mapping:
  XY:
    OneTwo:
      One: 1
      Two: 2
    X: x
    Y: y
  Foo: foo
  Bar: bar
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
    X: x
    # comment 8
    Y: y
  # comment 9
  Foo: foo
  # comment 10
  Bar: bar
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
    X: x
    {{- end }}
    # comment 8
    {{- if block 8 }}
    Y: y
    {{- end }}
  {{- end }}
  # comment 9
  {{- if block 9 }}
  Foo: foo
  {{- end }}
  # comment 10
  {{- if block 10 }}
  Bar: bar
  {{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmMappingOfList(t *testing.T) {
	list := NewEmptyList()
	list.AddNode(NewScalar("1"))
	list.AddNode(NewScalar("2"))
	list.AddNode(NewScalar("3"))

	root := NewNodeMapping("Mapping", NewNodeMapping("List", list))

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
	mapping := NewEmptyMapping()
	mapping.AddNode("Foo", NewScalar("foo"))
	mapping.AddNode("Bar", NewScalar("bar"))
	mapping.AddNode("Baz", NewScalar("baz"))

	list := NewEmptyList()
	list.AddNode(mapping)

	root := NewNodeMapping("Mapping", list)

	equal(t, root, `---
Mapping:
- Foo: foo
  Bar: bar
  Baz: baz
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Mapping:
# comment 3
- # comment 4
  Foo: foo
  # comment 5
  Bar: bar
  # comment 6
  Baz: baz
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
  Foo: foo
  {{- end }}
  # comment 5
  {{- if block 5 }}
  Bar: bar
  {{- end }}
  # comment 6
  {{- if block 6 }}
  Baz: baz
  {{- end }}
{{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmMultiLineComment(t *testing.T) {
	root := NewNodeMapping("Scalar", NewScalar("42", Comment("Many\n\nlines")))

	equal(t, root, `---
# Many
#
# lines
Scalar: 42
`)

	// list of list
	list1 := NewEmptyList()
	list1.AddNode(NewScalar("42", Comment("Many\n\nlines")))

	list2 := NewEmptyList()
	list2.AddNode(list1)
	list2.AddNode(NewScalar("foo"))

	root = NewNodeMapping("List", list2)

	equal(t, root, `---
List:
- # Many
  #
  # lines
  - 42
- foo
`)
}

func TestHelmWrapLongComments(t *testing.T) {
	root := NewEmptyMapping()
	mapping := NewEmptyMapping()
	word := "1"
	for i := len(word) + 1; i < 7; i++ {
		word += strconv.Itoa(i)
		root.AddNode(fmt.Sprintf("Key%d", i), NewScalar("~", Comment(strings.Repeat(word+" ", 10))))
		if i < 5 {
			mapping.AddNode(fmt.Sprintf("Key%d", i), NewScalar("~", Comment(strings.Repeat(word+" ", 5))))
		}
	}

	mapping.AddNode("Very", NewScalar("Long", Comment(strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.AddNode("Very", NewScalar("Long", Comment(strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.AddNode("Nested", mapping)

	expect := `---
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
Very: Long
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
          # xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
          # xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
          Very: Long
`
	equal(t, root, expect, Indent(10), Wrap(24))
}

func TestHelmIndent(t *testing.T) {
	mapping1 := NewEmptyMapping()
	mapping1.AddNode("Foo", NewScalar("Bar", Comment("Baz")))

	list1 := NewEmptyList()
	list1.AddNode(mapping1)
	list1.AddNode(NewScalar("1"))

	list2 := NewEmptyList()
	list2.AddNode(NewScalar("abc"))
	list2.AddNode(NewScalar("xyz"))

	list1.AddNode(list2)
	list1.AddNode(NewScalar("2"))
	list1.AddNode(NewScalar("3"))

	mapping2 := NewEmptyMapping()
	mapping2.AddNode("List", list1)

	mapping3 := NewEmptyMapping()
	mapping3.AddNode("Foo", NewScalar("1"))
	mapping3.AddNode("Bar", NewScalar("2"))

	mapping2.AddNode("Meta", mapping3)

	root := NewEmptyMapping()
	root.AddNode("Mapping", mapping2)

	expect := `---
Mapping:
    List:
      - # Baz
        Foo: Bar
      - 1
      -   - abc
          - xyz
      - 2
      - 3
    Meta:
        Foo: 1
        Bar: 2
`
	equal(t, root, expect, Indent(4))
}

func TestHelmEncoderModifier(t *testing.T) {
	mapping := NewEmptyMapping()
	mapping.AddNode("foo", NewScalar("1"))
	mapping.AddNode("bar", NewScalar("2"))
	mapping.AddNode("baz", NewScalar("3"))

	root := NewNodeMapping("Mapping", mapping)

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
	root := NewNodeMapping("Scalar", NewScalar("foo\nbar\nbaz"))
	list := NewNodeList(NewScalar("one\ntwo\nthree"))
	root.AddNode("List", list)

	expect := `---
Scalar: |-
    foo
    bar
    baz
List:
  - |-
    one
    two
    three
`
	equal(t, root, expect, Indent(4))
}

func TestHelmEmptyLines(t *testing.T) {
	list := NewEmptyList()
	list.AddNode(NewScalar("1", Comment("Some comment")))
	list.AddNode(NewScalar("2", Comment("")))
	list.AddNode(NewScalar("3", Comment("Another comment")))
	list.AddNode(NewScalar("4"))

	mapping := NewEmptyMapping(Comment("Mapping comment"))
	mapping.AddNode("List", list)
	mapping.AddNode("One", NewScalar("1", Comment("First post")))
	mapping.AddNode("Two", NewScalar("2"))
	mapping.AddNode("Three", NewScalar("3", Block("if .Values.set")))

	root := NewNodeMapping("Mapping", mapping, Comment("Top level comment"))

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

	list = NewEmptyList()
	list.AddNode(NewScalar("1"))
	list.AddNode(NewScalar("2", Comment("A comment")))
	list.AddNode(NewScalar("3"))

	root = NewNodeMapping("List", list)

	expect = `---
List:
- 1
# A comment
- 2
- 3
`
	equal(t, root, expect, EmptyLines(true))

	list.AddNode(NewScalar("4", Comment("Another comment")))
	root = NewNodeMapping("List", list)

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
	mapping := NewEmptyMapping()
	mapping.AddNode("foo", NewScalar("1"))
	mapping.AddNode("bar", NewScalar("2"))
	mapping.AddNode("baz", NewScalar("3"))

	root := NewNodeMapping("Mapping", mapping.Sort())

	equal(t, root, `---
Mapping:
  bar: 2
  baz: 3
  foo: 1
`)
	names := mapping.Names()
	assert.Equal(t, 3, len(names))
	assert.Equal(t, "foo", names[2])
	assert.Equal(t, "1", mapping.Get(names[2]).Value())
}

func TestHelmError(t *testing.T) {
	root := NewNodeMapping("Foo", NewScalar("1"))

	buffer := &bytes.Buffer{}
	enc := NewEncoder(buffer)
	enc.err = errors.New("monkey wrench")

	assert.NotNil(t, enc.Encode(root))
	assert.Equal(t, "", buffer.String())
}

func TestHelmMerge(t *testing.T) {
	root := NewNodeMapping("One", NewScalar("1"))
	root.Merge(NewNodeMapping("Foo", NewScalar("2")))

	equal(t, root, `---
One: 1
Foo: 2
`)
}

func TestHelmAddValues(t *testing.T) {
	root := NewMapping("One", "1", "Two", "2", "Three")
	root.AddNode("List", NewList("X", "Y"))
	root.AddInt("Int", 42, Block("if condition"))
	root.AddNode("Nodes", NewNodeList(NewScalar("foo"), NewScalar("bar")))
	equal(t, root, `---
One: 1
Two: 2
Three: ~
List:
- X
- Y
{{- if condition }}
Int: 42
{{- end }}
Nodes:
- foo
- bar
`)
}

func TestHelmAddNilNodes(t *testing.T) {
	root := NewEmptyMapping()
	root.AddNode("Foo", NewScalar("X"))
	root.AddNode("Bar", nil)
	root.AddNode("Baz", NewScalar("Y"))
	root.AddNode("List", NewNodeList(NewScalar("1"), nil, NewScalar("2")))
	equal(t, root, `---
Foo: X
Baz: Y
List:
- 1
- 2
`)
}

func TestHelmGetNode(t *testing.T) {
	root := NewMapping("Foo", "1", "Bar", "2")
	assert.Nil(t, root.Get("Baz"))

	bar := root.Get("Bar")
	assert.NotNil(t, bar)

	if bar != nil {
		assert.Equal(t, bar.Value(), "2")
		bar.SetValue("3")
	}

	root.AddNode("Baz", NewMapping("xyzzy", "plugh"))
	assert.Equal(t, "plugh", root.Get("Baz").Get("xyzzy").Value())

	equal(t, root, `---
Foo: 1
Bar: 3
Baz:
  xyzzy: plugh
`)
}

func TestHelmEncodeList(t *testing.T) {
	root := NewList("One", "Two")
	equal(t, root, `---
- One
- Two
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
	assert.False(t, hasPanicked(func() { NewScalar("foo").Value() }))
	assert.True(t, hasPanicked(func() { NewEmptyMapping().Value() }))
	assert.True(t, hasPanicked(func() { NewEmptyMapping().Value() }))

	assert.False(t, hasPanicked(func() { NewScalar("foo").SetValue("new") }))
	assert.True(t, hasPanicked(func() { NewEmptyList().SetValue("new") }))
	assert.True(t, hasPanicked(func() { NewEmptyMapping().SetValue("new") }))

	assert.True(t, hasPanicked(func() { NewScalar("foo").Values() }))
	assert.False(t, hasPanicked(func() { NewEmptyList().Values() }))
	assert.True(t, hasPanicked(func() { NewEmptyMapping().Values() }))

	assert.True(t, hasPanicked(func() { NewScalar("foo").Get("key") }))
	assert.True(t, hasPanicked(func() { NewEmptyList().Get("key") }))
	assert.False(t, hasPanicked(func() { NewEmptyMapping().Get("key") }))
}
