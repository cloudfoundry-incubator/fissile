package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SUSE/fissile/testhelpers"
)

func TestNewTypeMeta(t *testing.T) {
	assert := assert.New(t)

	typeMeta := newTypeMeta("the-api-version", "thekind")

	actual, err := testhelpers.RoundtripKube(typeMeta)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "the-api-version"
		kind: "thekind"
	`, actual)
}

func TestNewObjectMeta(t *testing.T) {
	assert := assert.New(t)

	objectMeta := newObjectMeta("thename")

	actual, err := testhelpers.RoundtripKube(objectMeta)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		name: "thename"
		labels:
			skiff-role-name: "thename"
	`, actual)
}

func TestNewSelector(t *testing.T) {
	assert := assert.New(t)

	selector := newSelector("thename")

	actual, err := testhelpers.RoundtripKube(selector)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		matchLabels:
			skiff-role-name: "thename"
	`, actual)
}

func TestNewKubeConfig(t *testing.T) {
	assert := assert.New(t)

	kubeConfig := newKubeConfig("theApiVersion", "thekind", "thename")

	actual, err := testhelpers.RoundtripKube(kubeConfig)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "theApiVersion"
		kind: "thekind"
		metadata:
			name: "thename"
			labels:
				skiff-role-name: "thename"
	`, actual)
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
