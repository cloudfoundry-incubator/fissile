package kube

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"
)

func TestMakeSecretsEmpty(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	expected := `---
		apiVersion: "v1"
		data: {}
		kind: "Secret"
		metadata:
			name: "secrets"
			labels:
				skiff-role-name: "secrets"
	`

	t.Run("Kube", func(t *testing.T) {
		t.Parallel()
		secret, err := MakeSecrets(model.CVMap{}, ExportSettings{})
		if !assert.NoError(err) {
			return
		}
		actual, err := testhelpers.RoundtripKube(secret)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, expected, actual)
	})

	t.Run("Helm", func(t *testing.T) {
		t.Parallel()
		secret, err := MakeSecrets(model.CVMap{}, ExportSettings{
			CreateHelmChart: true,
		})
		if !assert.NoError(err) {
			return
		}
		actual, err := testhelpers.RoundtripNode(secret, nil)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, expected, actual)
	})
}

// name
// cv.Description
// cv.Default		nil/!nil
// cv.Generator	helm	nil/!nil	(ID, Type (SSH|Passwpord|(CA)Certificate), ValueType)
// cv.Immutable	helm	true/false
// cv.Name	helm

func testCVMap() model.CVMap {
	return model.CVMap{
		"min": &model.ConfigurationVariable{
			Name: "min",
		},
		"desc": &model.ConfigurationVariable{
			Name:        "desc",
			Description: "<<<a description>>>",
		},
		"valued": &model.ConfigurationVariable{
			Name:        "valued",
			Description: "<<<invaluable>>>",
			Default:     "you are very valued indeed",
		},
		"const": &model.ConfigurationVariable{
			Name:        "const",
			Description: "<<<don't change>>>",
			Default:     "rock solid",
			Immutable:   true,
		},
		"genie": &model.ConfigurationVariable{
			Name:        "genie",
			Description: "<<<here is jeannie>>>",
			Generator: &model.ConfigurationVariableGenerator{
				ID:        "xxx",
				Type:      model.GeneratorTypePassword,
				ValueType: "snafu",
			},
		},
		"guinevere": &model.ConfigurationVariable{
			// excluded from __helm__ output
			Name:        "guinevere",
			Description: "<<<helm hidden>>>",
			Immutable:   true,
			Generator: &model.ConfigurationVariableGenerator{
				ID:        "xxx",
				Type:      model.GeneratorTypePassword,
				ValueType: "snafu",
			},
		},
	}
}

func TestMakeSecretsKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCV := testCVMap()

	secret, err := MakeSecrets(testCV, ExportSettings{})
	if !assert.NoError(err) {
		return
	}

	renderedYAML, err := testhelpers.RenderNode(secret, nil)
	if !assert.NoError(err) {
		return
	}

	varConstB64 := testhelpers.RenderEncodeBase64(testCV["const"].Default.(string))
	varValuedB64 := testhelpers.RenderEncodeBase64(testCV["valued"].Default.(string))

	// Check the comments, and also that they are associated with
	// the correct variables.

	astring := string(renderedYAML)

	assert.Contains(astring, fmt.Sprintf("# <<<don't change>>>\n  const: \"%s\"", varConstB64))
	assert.Contains(astring, "# <<<a description>>>\n  desc: \"\"")
	assert.Contains(astring, "\n  min: \"\"")
	assert.Contains(astring, fmt.Sprintf("# <<<invaluable>>>\n  valued: \"%s\"", varValuedB64))
	assert.Contains(astring, "# <<<here is jeannie>>>\n  genie: \"\"")
	assert.Contains(astring, "# <<<helm hidden>>>\n  guinevere: \"\"")

	actual, err := testhelpers.RoundtripKube(secret)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, fmt.Sprintf(`---
		apiVersion: "v1"
		data:
			const: "%s"
			desc: ""
			genie: ""
			guinevere: ""
			min: ""
			valued: "%s"
		kind: "Secret"
		metadata:
			name: "secrets"
			labels:
				skiff-role-name: "secrets"
	`, varConstB64, varValuedB64), actual)
}

func TestMakeSecretsHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	secret, err := MakeSecrets(testCVMap(), ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}

	t.Run("Missing", func(t *testing.T) {
		t.Parallel()
		// config - helm only
		// Render with defaults (is expected to) fail(s) due
		// to a number of guards (secrets.FOO, FOO a variable)
		// not being present at all.

		_, err := testhelpers.RenderNode(secret, nil)
		assert.EqualError(err,
			`template: :6:12: executing "" at <required "secrets.co...>: error calling required: secrets.const has not been set`)
	})

	t.Run("Undefined", func(t *testing.T) {
		t.Parallel()
		// config - helm only
		// Render with defaults (is expected to) fail(s) due
		// to a number of guards (secrets.FOO, FOO a variable)
		// not having a proper (non-nil) value.

		config := map[string]interface{}{
			"Values.secrets.const": nil,
		}

		_, err := testhelpers.RenderNode(secret, config)
		assert.EqualError(err,
			`template: :6:12: executing "" at <required "secrets.co...>: error calling required: secrets.const has not been set`)
	})

	t.Run("Present", func(t *testing.T) {
		t.Parallel()
		varConst := "cannot change"
		varDesc := "self-explanatory"
		varMin := "rock-bottom"
		varValued := "sky high"
		varGenie := "djinn"

		varConstB64 := testhelpers.RenderEncodeBase64(varConst)
		varDescB64 := testhelpers.RenderEncodeBase64(varDesc)
		varMinB64 := testhelpers.RenderEncodeBase64(varMin)
		varValuedB64 := testhelpers.RenderEncodeBase64(varValued)
		varGenieB64 := testhelpers.RenderEncodeBase64(varGenie)

		config := map[string]interface{}{
			"Values.secrets.const":  varConst,
			"Values.secrets.desc":   varDesc,
			"Values.secrets.min":    varMin,
			"Values.secrets.valued": varValued,
			"Values.secrets.genie":  varGenie,
		}

		renderedYAML, err := testhelpers.RenderNode(secret, config)
		if !assert.NoError(err) {
			return
		}

		// Check the comments, and also that they are associated with
		// the correct variables.

		astring := string(renderedYAML)

		assert.Contains(astring, "# <<<don't change>>>\n  # This value is")
		assert.Contains(astring, fmt.Sprintf("# This value is immutable and must not be changed once set.\n  const: \"%s\"", varConstB64))
		assert.Contains(astring, fmt.Sprintf("# <<<a description>>>\n  desc: \"%s\"", varDescB64))
		assert.Contains(astring, fmt.Sprintf("\n  min: \"%s\"", varMinB64))
		assert.Contains(astring, fmt.Sprintf("# <<<invaluable>>>\n  valued: \"%s\"", varValuedB64))
		assert.Contains(astring, "# <<<here is jeannie>>>\n  # This value uses ")
		assert.Contains(astring, fmt.Sprintf("# This value uses a generated default.\n  genie: \"%s\"", varGenieB64))

		// And check overall structure

		actual, err := testhelpers.RoundtripNode(secret, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, fmt.Sprintf(`---
			apiVersion: "v1"
			data:
				const: "%s"
				desc: "%s"
				min: "%s"
				valued: "%s"
				genie: "%s"
			kind: "Secret"
			metadata:
				name: "secrets"
				labels:
					skiff-role-name: "secrets"
		`, varConstB64, varDescB64, varMinB64, varValuedB64, varGenieB64), actual)
	})
}
