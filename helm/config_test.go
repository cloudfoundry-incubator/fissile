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
	case *Object:
		for _, namedNode := range node.(*Object).nodes {
			index = annotate(namedNode.node, comment, index)
		}
	}
	return index
}
func addComments(node Node) { annotate(node, true, 0) }
func addBlocks(node Node)   { annotate(node, false, 0) }

func equal(t *testing.T, config *Object, expect string, modifiers ...func(*Encoder)) {
	buffer := &bytes.Buffer{}
	assert.Nil(t, NewEncoder(buffer, modifiers...).Encode(config))
	assert.Equal(t, expect, buffer.String())
}

func TestHelmScalar(t *testing.T) {
	root := NewObject()
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

	root := NewObject()
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

func TestHelmObject(t *testing.T) {
	obj := NewObject()
	obj.Add("foo", NewScalar("1"))
	obj.Add("bar", NewScalar("2"))
	obj.Add("baz", NewScalar("3"))

	root := NewObject()
	root.Add("Object", obj)

	equal(t, root, `---
Object:
  foo: 1
  bar: 2
  baz: 3
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Object:
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
Object:
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

	root := NewObject()
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

func TestHelmObjectOfObject(t *testing.T) {
	obj1 := NewObject()
	obj1.Add("One", NewScalar("1"))
	obj1.Add("Two", NewScalar("2"))

	obj2 := NewObject()
	obj2.Add("OneTwo", obj1)
	obj2.Add("X", NewScalar("x"))
	obj2.Add("Y", NewScalar("y"))

	obj3 := NewObject()
	obj3.Add("XY", obj2)
	obj3.Add("Foo", NewScalar("foo"))
	obj3.Add("Bar", NewScalar("bar"))

	root := NewObject()
	root.Add("Object", obj3)

	equal(t, root, `---
Object:
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
Object:
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
Object:
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

func TestHelmObjectOfList(t *testing.T) {
	list := NewList()
	list.Add(NewScalar("1"))
	list.Add(NewScalar("2"))
	list.Add(NewScalar("3"))

	obj := NewObject()
	obj.Add("List", list)

	root := NewObject()
	root.Add("Object", obj)

	equal(t, root, `---
Object:
  List:
  - 1
  - 2
  - 3
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Object:
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
Object:
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

func TestHelmListOfObject(t *testing.T) {
	obj := NewObject()
	obj.Add("Foo", NewScalar("foo"))
	obj.Add("Bar", NewScalar("bar"))
	obj.Add("Baz", NewScalar("baz"))

	list := NewList()
	list.Add(obj)

	root := NewObject()
	root.Add("Object", list)

	equal(t, root, `---
Object:
- Foo: foo
  Bar: bar
  Baz: baz
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Object:
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
Object:
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
	root := NewObject()
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

	root = NewObject()
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
	root := NewObject()
	obj := NewObject()
	word := "1"
	for i := len(word) + 1; i < 7; i++ {
		word += strconv.Itoa(i)
		root.Add(fmt.Sprintf("Key%d", i), NewScalar("~", Comment(strings.Repeat(word+" ", 10))))
		if i < 5 {
			obj.Add(fmt.Sprintf("Key%d", i), NewScalar("~", Comment(strings.Repeat(word+" ", 5))))
		}
	}

	obj.Add("Very", NewScalar("Long", Comment(strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.Add("Very", NewScalar("Long", Comment(strings.Repeat(strings.Repeat("x", 50)+" ", 2))))
	root.Add("Nested", obj)

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
	obj1 := NewObject()
	obj1.Add("Foo", NewScalar("Bar", Comment("Baz")))

	list1 := NewList()
	list1.Add(obj1)
	list1.Add(NewScalar("1"))

	list2 := NewList()
	list2.Add(NewScalar("abc"))
	list2.Add(NewScalar("xyz"))

	list1.Add(list2)
	list1.Add(NewScalar("2"))
	list1.Add(NewScalar("3"))

	obj2 := NewObject()
	obj2.Add("List", list1)

	obj3 := NewObject()
	obj3.Add("Foo", NewScalar("1"))
	obj3.Add("Bar", NewScalar("2"))

	obj2.Add("Meta", obj3)

	root := NewObject()
	root.Add("Object", obj2)

	expect := `---
Object:
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
	obj := NewObject()
	obj.Add("foo", NewScalar("1"))
	obj.Add("bar", NewScalar("2"))
	obj.Add("baz", NewScalar("3"))

	root := NewObject()
	root.Add("Object", obj)

	expect := `---
Object:
  foo: 1
  bar: 2
  baz: 3
---
Object:
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
	root := NewObject()
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

	obj := NewObject()
	obj.Add("List", list)
	obj.Add("One", NewScalar("1", Comment("First post")))
	obj.Add("Two", NewScalar("2"))
	obj.Add("Three", NewScalar("3", Block("if .Values.set")))

	root := NewObject()
	root.Add("Object", obj)

	expect := `---
Object:
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
Object:
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

	root = NewObject()
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

func TestHelmObjectSort(t *testing.T) {
	obj := NewObject()
	obj.Add("foo", NewScalar("1"))
	obj.Add("bar", NewScalar("2"))
	obj.Add("baz", NewScalar("3"))
	obj.Sort()

	root := NewObject()
	root.Add("Object", obj)

	equal(t, root, `---
Object:
  bar: 2
  baz: 3
  foo: 1
`)
}

func TestHelmError(t *testing.T) {
	root := NewObject()
	root.Add("Foo", NewScalar("1"))

	buffer := &bytes.Buffer{}
	enc := NewEncoder(buffer)
	enc.err = errors.New("monkey wrench")

	assert.NotNil(t, enc.Encode(root))
	assert.Equal(t, "", buffer.String())
}
