package kube

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// annotate by recursively adding comments or conditions to each node
func annotate(any ConfigType, comment bool, index int) int {
	index++
	if comment {
		any.setComment(fmt.Sprintf("comment %d", index))
	} else {
		any.setCondition(fmt.Sprintf("if condition %d", index))
	}
	switch any.(type) {
	case *ConfigList:
		for _, item := range any.(*ConfigList).values {
			index = annotate(item, comment, index)
		}
	case *ConfigObject:
		for _, item := range any.(*ConfigObject).values {
			index = annotate(item.value, comment, index)
		}
	}
	return index
}
func addComments(any ConfigType)   { annotate(any, true, 0) }
func addConditions(any ConfigType) { annotate(any, false, 0) }

func equal(t *testing.T, config *ConfigObject, expect string) {
	buffer := &bytes.Buffer{}
	config.WriteConfig(buffer)
	assert.Equal(t, expect, buffer.String())
}

func TestConfig(t *testing.T) {

	// Simple scalar
	root := &ConfigObject{}
	root.add("Scalar", NewConfigScalar("42"))

	equal(t, root, `---
Scalar: 42
`)

	addComments(root)
	equal(t, root, `---
# comment 1
# comment 2
Scalar: 42
`)

	addConditions(root)
	equal(t, root, `---
# comment 1
{{- if condition 1 }}
# comment 2
{{- if condition 2 }}
Scalar: 42
{{- end }}
{{- end }}
`)

	// Simple list
	list := &ConfigList{}
	list.add(NewConfigScalar("1"))
	list.add(NewConfigScalar("2"))
	list.add(NewConfigScalar("3"))

	root = &ConfigObject{}
	root.add("List", list)

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

	addConditions(root)
	equal(t, root, `---
# comment 1
{{- if condition 1 }}
# comment 2
{{- if condition 2 }}
List:
# comment 3
{{- if condition 3 }}
- 1
{{- end }}
# comment 4
{{- if condition 4 }}
- 2
{{- end }}
# comment 5
{{- if condition 5 }}
- 3
{{- end }}
{{- end }}
{{- end }}
`)

	// Simple Object
	obj := &ConfigObject{}
	obj.add("foo", NewConfigScalar("1"))
	obj.add("bar", NewConfigScalar("2"))
	obj.add("baz", NewConfigScalar("3"))

	root = &ConfigObject{}
	root.add("Object", obj)

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

	addConditions(root)
	equal(t, root, `---
# comment 1
{{- if condition 1 }}
# comment 2
{{- if condition 2 }}
Object:
  # comment 3
  {{- if condition 3 }}
  foo: 1
  {{- end }}
  # comment 4
  {{- if condition 4 }}
  bar: 2
  {{- end }}
  # comment 5
  {{- if condition 5 }}
  baz: 3
  {{- end }}
{{- end }}
{{- end }}
`)

	// list of list of list
	list1 := &ConfigList{}
	list1.add(NewConfigScalar("1"))
	list1.add(NewConfigScalar("2"))

	list2 := &ConfigList{}
	list2.add(list1)
	list2.add(NewConfigScalar("x"))
	list2.add(NewConfigScalar("y"))

	list3 := &ConfigList{}
	list3.add(list2)
	list3.add(NewConfigScalar("foo"))
	list3.add(NewConfigScalar("bar"))

	root = &ConfigObject{}
	root.add("List", list3)

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

	addConditions(root)
	equal(t, root, `---
# comment 1
{{- if condition 1 }}
# comment 2
{{- if condition 2 }}
List:
# comment 3
{{- if condition 3 }}
- # comment 4
  {{- if condition 4 }}
  - # comment 5
    {{- if condition 5 }}
    - 1
    {{- end }}
    # comment 6
    {{- if condition 6 }}
    - 2
    {{- end }}
  {{- end }}
  # comment 7
  {{- if condition 7 }}
  - x
  {{- end }}
  # comment 8
  {{- if condition 8 }}
  - y
  {{- end }}
{{- end }}
# comment 9
{{- if condition 9 }}
- foo
{{- end }}
# comment 10
{{- if condition 10 }}
- bar
{{- end }}
{{- end }}
{{- end }}
`)

	// Object of object of object
	obj1 := &ConfigObject{}
	obj1.add("One", NewConfigScalar("1"))
	obj1.add("Two", NewConfigScalar("2"))

	obj2 := &ConfigObject{}
	obj2.add("OneTwo", obj1)
	obj2.add("X", NewConfigScalar("x"))
	obj2.add("Y", NewConfigScalar("y"))

	obj3 := &ConfigObject{}
	obj3.add("XY", obj2)
	obj3.add("Foo", NewConfigScalar("foo"))
	obj3.add("Bar", NewConfigScalar("bar"))

	root = &ConfigObject{}
	root.add("Object", obj3)

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

	addConditions(root)
	equal(t, root, `---
# comment 1
{{- if condition 1 }}
# comment 2
{{- if condition 2 }}
Object:
  # comment 3
  {{- if condition 3 }}
  XY:
    # comment 4
    {{- if condition 4 }}
    OneTwo:
      # comment 5
      {{- if condition 5 }}
      One: 1
      {{- end }}
      # comment 6
      {{- if condition 6 }}
      Two: 2
      {{- end }}
    {{- end }}
    # comment 7
    {{- if condition 7 }}
    X: x
    {{- end }}
    # comment 8
    {{- if condition 8 }}
    Y: y
    {{- end }}
  {{- end }}
  # comment 9
  {{- if condition 9 }}
  Foo: foo
  {{- end }}
  # comment 10
  {{- if condition 10 }}
  Bar: bar
  {{- end }}
{{- end }}
{{- end }}
`)

	// Object of list
	list = &ConfigList{}
	list.add(NewConfigScalar("1"))
	list.add(NewConfigScalar("2"))
	list.add(NewConfigScalar("3"))

	obj = &ConfigObject{}
	obj.add("List", list)

	root = &ConfigObject{}
	root.add("Object", obj)

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

	addConditions(root)
	equal(t, root, `---
# comment 1
{{- if condition 1 }}
# comment 2
{{- if condition 2 }}
Object:
  # comment 3
  {{- if condition 3 }}
  List:
  # comment 4
  {{- if condition 4 }}
  - 1
  {{- end }}
  # comment 5
  {{- if condition 5 }}
  - 2
  {{- end }}
  # comment 6
  {{- if condition 6 }}
  - 3
  {{- end }}
  {{- end }}
{{- end }}
{{- end }}
`)

	// List of Object
	obj = &ConfigObject{}
	obj.add("Foo", NewConfigScalar("foo"))
	obj.add("Bar", NewConfigScalar("bar"))
	obj.add("Baz", NewConfigScalar("baz"))

	list = &ConfigList{}
	list.add(obj)

	root = &ConfigObject{}
	root.add("Object", list)

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

	addConditions(root)
	equal(t, root, `---
# comment 1
{{- if condition 1 }}
# comment 2
{{- if condition 2 }}
Object:
# comment 3
{{- if condition 3 }}
- # comment 4
  {{- if condition 4 }}
  Foo: foo
  {{- end }}
  # comment 5
  {{- if condition 5 }}
  Bar: bar
  {{- end }}
  # comment 6
  {{- if condition 6 }}
  Baz: baz
  {{- end }}
{{- end }}
{{- end }}
{{- end }}
`)

	// Multi-line comments
	root = &ConfigObject{}
	scalar := NewConfigScalar("42")
	scalar.setComment("Many\n\nlines")
	root.add("Scalar", scalar)

	equal(t, root, `---
# Many
#
# lines
Scalar: 42
`)

	// list of list
	list1 = &ConfigList{}
	scalar = NewConfigScalar("42")
	scalar.setComment("Many\n\nlines")
	list1.add(scalar)

	list2 = &ConfigList{}
	list2.add(list1)
	list2.add(NewConfigScalar("foo"))

	root = &ConfigObject{}
	root.add("List", list2)

	equal(t, root, `---
List:
- # Many
  #
  # lines
  - 42
- foo
`)

}
