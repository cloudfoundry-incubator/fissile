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

func equal(t *testing.T, config *Mapping, expect string, modifiers ...func(*Encoder)) {
	buffer := &bytes.Buffer{}
	assert.Nil(t, NewEncoder(buffer, modifiers...).Encode(config))
	assert.Equal(t, expect, buffer.String())
}

func TestHelmScalar(t *testing.T) {
	root := NewMapping()
	root.Add("Scalar", NewScalar("42"))

	equal(t, root, `---
Scalar: 42
`)

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
	list := NewList()
	list.Add(NewScalar("1"))
	list.Add(NewScalar("2"))
	list.Add(NewScalar("3"))

	root := NewMapping()
	root.Add("List", list)

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
}

func TestHelmMapping(t *testing.T) {
	mapping := NewMapping()
	mapping.Add("foo", NewScalar("1"))
	mapping.Add("bar", NewScalar("2"))
	mapping.Add("baz", NewScalar("3"))

	root := NewMapping()
	root.Add("Mapping", mapping)

	equal(t, root, `---
Mapping:
  foo: 1
  bar: 2
  baz: 3
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Mapping:
  # comment 3
  foo: 1
  # comment 4
  bar: 2
  # comment 5
  baz: 3
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
  bar: 2
  {{- end }}
  # comment 5
  {{- if block 5 }}
  baz: 3
  {{- end }}
{{- end }}
{{- end }}
`)
}

func TestHelmListOfList(t *testing.T) {
	list1 := NewList()
	list1.Add(NewScalar("1"))
	list1.Add(NewScalar("2"))

	list2 := NewList()
	list2.Add(list1)
	list2.Add(NewScalar("x"))
	list2.Add(NewScalar("y"))

	list3 := NewList()
	list3.Add(list2)
	list3.Add(NewScalar("foo"))
	list3.Add(NewScalar("bar"))

	root := NewMapping()
	root.Add("List", list3)

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
	mapping1 := NewMapping()
	mapping1.Add("One", NewScalar("1"))
	mapping1.Add("Two", NewScalar("2"))

	mapping2 := NewMapping()
	mapping2.Add("OneTwo", mapping1)
	mapping2.Add("X", NewScalar("x"))
	mapping2.Add("Y", NewScalar("y"))

	mapping3 := NewMapping()
	mapping3.Add("XY", mapping2)
	mapping3.Add("Foo", NewScalar("foo"))
	mapping3.Add("Bar", NewScalar("bar"))

	root := NewMapping()
	root.Add("Mapping", mapping3)

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
	list := NewList()
	list.Add(NewScalar("1"))
	list.Add(NewScalar("2"))
	list.Add(NewScalar("3"))

	mapping := NewMapping()
	mapping.Add("List", list)

	root := NewMapping()
	root.Add("Mapping", mapping)

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
	mapping.Add("Foo", NewScalar("foo"))
	mapping.Add("Bar", NewScalar("bar"))
	mapping.Add("Baz", NewScalar("baz"))

	list := NewList()
	list.Add(mapping)

	root := NewMapping()
	root.Add("Mapping", list)

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
	root := NewMapping()
	root.Add("Scalar", NewScalar("42", Comment("Many\n\nlines")))

	equal(t, root, `---
# Many
#
# lines
Scalar: 42
`)

	// list of list
	list1 := NewList()
	list1.Add(NewScalar("42", Comment("Many\n\nlines")))

	list2 := NewList()
	list2.Add(list1)
	list2.Add(NewScalar("foo"))

	root = NewMapping()
	root.Add("List", list2)

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
	root := NewMapping()
	mapping := NewMapping()
	word := "1"
	for i := len(word) + 1; i < 7; i++ {
		word += strconv.Itoa(i)
		root.Add(fmt.Sprintf("Key%d", i), NewScalar("~", Comment(strings.Repeat(word+" ", 10))))
		if i < 5 {
			mapping.Add(fmt.Sprintf("Key%d", i), NewScalar("~", Comment(strings.Repeat(word+" ", 5))))
		}
	}

	mapping.Add("Very", NewScalar("Long", Comment(strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.Add("Very", NewScalar("Long", Comment(strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.Add("Nested", mapping)

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
	mapping1 := NewMapping()
	mapping1.Add("Foo", NewScalar("Bar", Comment("Baz")))

	list1 := NewList()
	list1.Add(mapping1)
	list1.Add(NewScalar("1"))

	list2 := NewList()
	list2.Add(NewScalar("abc"))
	list2.Add(NewScalar("xyz"))

	list1.Add(list2)
	list1.Add(NewScalar("2"))
	list1.Add(NewScalar("3"))

	mapping2 := NewMapping()
	mapping2.Add("List", list1)

	mapping3 := NewMapping()
	mapping3.Add("Foo", NewScalar("1"))
	mapping3.Add("Bar", NewScalar("2"))

	mapping2.Add("Meta", mapping3)

	root := NewMapping()
	root.Add("Mapping", mapping2)

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
	mapping := NewMapping()
	mapping.Add("foo", NewScalar("1"))
	mapping.Add("bar", NewScalar("2"))
	mapping.Add("baz", NewScalar("3"))

	root := NewMapping()
	root.Add("Mapping", mapping)

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
	root := NewMapping()
	root.Add("Scalar", NewScalar("foo\nbar\nbaz"))

	list := NewList()
	list.Add(NewScalar("one\ntwo\nthree"))
	root.Add("List", list)

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
	list := NewList()
	list.Add(NewScalar("1", Comment("Some comment")))
	list.Add(NewScalar("2", Comment("")))
	list.Add(NewScalar("3", Comment("Another comment")))
	list.Add(NewScalar("4"))

	mapping := NewMapping()
	mapping.Add("List", list)
	mapping.Add("One", NewScalar("1", Comment("First post")))
	mapping.Add("Two", NewScalar("2"))
	mapping.Add("Three", NewScalar("3", Block("if .Values.set")))

	root := NewMapping()
	root.Add("Mapping", mapping)

	expect := `---
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
	list.Add(NewScalar("1"))
	list.Add(NewScalar("2", Comment("A comment")))
	list.Add(NewScalar("3"))

	root = NewMapping()
	root.Add("List", list)

	expect = `---
List:
- 1

# A comment
- 2

- 3
`
	equal(t, root, expect, EmptyLines(true))
}

func TestHelmMappingSort(t *testing.T) {
	mapping := NewMapping()
	mapping.Add("foo", NewScalar("1"))
	mapping.Add("bar", NewScalar("2"))
	mapping.Add("baz", NewScalar("3"))
	mapping.Sort()

	root := NewMapping()
	root.Add("Mapping", mapping)

	equal(t, root, `---
Mapping:
  bar: 2
  baz: 3
  foo: 1
`)
}

func TestHelmError(t *testing.T) {
	root := NewMapping()
	root.Add("Foo", NewScalar("1"))

	buffer := &bytes.Buffer{}
	enc := NewEncoder(buffer)
	enc.err = errors.New("monkey wrench")

	assert.NotNil(t, enc.Encode(root))
	assert.Equal(t, "", buffer.String())
}
