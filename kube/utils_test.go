package kube

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestNewTypeMeta(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	typeMeta := newTypeMeta("the-api-version", "thekind")

	actual, err := RoundtripKube(typeMeta)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "the-api-version"
		kind: "thekind"
	`, actual)
}

func TestNewSelector(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	makeTemplateRole := func() *model.InstanceGroup {
		return &model.InstanceGroup{
			Name: "thename",
		}
	}
	role := makeTemplateRole()
	settings := ExportSettings{
		IstioComplied: false,
	}
	selector := newSelector(role, settings)

	actual, err := RoundtripKube(selector)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		matchLabels:
			skiff-role-name: "thename"
	`, actual)
}

func TestNewSelectorIstioManaged(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	makeTemplateRole := func() *model.InstanceGroup {
		return &model.InstanceGroup{
			Name: "thename",
			Tags: []model.RoleTag{model.RoleTagIstioManaged},
		}
	}
	role := makeTemplateRole()
	settings := ExportSettings{
		IstioComplied: true,
		CreateHelmChart: true,
	}
	selector := newSelector(role, settings)

	actual, err := RoundtripNode(selector, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		matchLabels:
			skiff-role-name: "thename"
			app: "thename"
			version: 1.22.333.4444
	`, actual)
}

func TestNewKubeConfig(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	kubeConfig := newKubeConfig(ExportSettings{}, "theApiVersion", "thekind", "thename")

	actual, err := RoundtripKube(kubeConfig)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "theApiVersion"
		kind: "thekind"
		metadata:
			name: "thename"
			labels:
				app.kubernetes.io/component: "thename"
	`, actual)
}

func TestMakeVarName(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	assert := assert.New(t)

	// The template condition we wish to check: 3.1 <= version
	vCheck := minKubeVersion(3, 1)

	// Wrap it into a proper node we can render. Depending on the
	// outcome of the version check one of the two list children
	// below will be visible. Which of them is the outcome we are
	// testing for.
	vMatched := helm.Block(fmt.Sprintf("if (%s)", vCheck))
	vMisMatch := helm.Block(fmt.Sprintf("if not (%s)", vCheck))
	v := helm.NewList()
	v.Add(helm.NewNode("match", vMatched))
	v.Add(helm.NewNode("no-match", vMisMatch))

	for _, testcase := range []struct {
		Major  string
		Minor  string
		Result string
	}{
		{"2", "0", "- no-match"},
		{"3", "0", "- no-match"},
		{"3", "1", "- match"},
		{"3", "2", "- match"},
		{"4", "0", "- match"},
	} {
		config := map[string]interface{}{
			"Capabilities.KubeVersion.Major": testcase.Major,
			"Capabilities.KubeVersion.Minor": testcase.Minor,
		}
		actual, err := RoundtripNode(v, config)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, testcase.Result, actual)
	}
}
