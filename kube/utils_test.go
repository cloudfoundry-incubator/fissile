package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SUSE/fissile/testhelpers"
)

func TestNewTypeMeta(t *testing.T) {
	assert := assert.New(t)

	typemeta := newTypeMeta("the-api-version", "thekind")
	typemetaYAML, err := testhelpers.RenderNode(typemeta, nil)
	if !assert.NoError(err) {
		return
	}

	expectedYAML := `---
apiVersion: "the-api-version"
kind: "thekind"
`
	assert.Equal(expectedYAML, string(typemetaYAML))
}

func TestNewObjectMeta(t *testing.T) {
	assert := assert.New(t)

	meta := newObjectMeta("thename")
	metaYAML, err := testhelpers.RenderNode(meta, nil)
	if !assert.NoError(err) {
		return
	}

	expectedYAML := `---
name: "thename"
labels:
  skiff-role-name: "thename"
`
	assert.Equal(expectedYAML, string(metaYAML))
}

func TestNewSelector(t *testing.T) {
	assert := assert.New(t)

	sel := newSelector("thename")
	selYAML, err := testhelpers.RenderNode(sel, nil)
	if !assert.NoError(err) {
		return
	}

	expectedYAML := `---
matchLabels:
  skiff-role-name: "thename"
`
	assert.Equal(expectedYAML, string(selYAML))
}

func TestNewKubeConfig(t *testing.T) {
	assert := assert.New(t)

	kc := newKubeConfig("theApiVersion", "thekind", "thename")

	kcYAML, err := testhelpers.RenderNode(kc, nil)
	if !assert.NoError(err) {
		return
	}

	expectedYAML := `---
apiVersion: "theApiVersion"
kind: "thekind"
metadata:
  name: "thename"
  labels:
    skiff-role-name: "thename"
`
	assert.Equal(expectedYAML, string(kcYAML))
}

func TestMakeVarName(t *testing.T) {
	assert := assert.New(t)

	testcases := []struct {
		name    string
		varname string
	}{
		{"", ""},
		{"a", "a"},
		{"a-foo", "a_foo"},
		{"a-foo-bar", "a_foo_bar"},
		{"-", "_"},
		{"a_-b", "a__b"},
	}
	for _, acase := range testcases {
		assert.Equal(acase.varname, makeVarName(acase.name))
	}
}

func TestMinKubeVersion(t *testing.T) {
	assert := assert.New(t)

	expected := `or (gt (int .Capabilities.KubeVersion.Major) 3) (and (eq (int .Capabilities.KubeVersion.Major) 3) (ge (.Capabilities.KubeVersion.Minor | trimSuffix "+" | int) 1))`

	assert.Equal(expected, minKubeVersion(3, 1))
}
