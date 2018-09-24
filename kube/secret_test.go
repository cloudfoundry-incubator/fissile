package kube

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudfoundry-incubator/fissile/model"
	"github.com/cloudfoundry-incubator/fissile/testhelpers"
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
		actual, err := RoundtripKube(secret)
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
		actual, err := RoundtripNode(secret, nil)
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
		"optional": &model.VariableDefinition{
			Name: "optional",
			CVOptions: model.CVOptions{
				// This variable is only defined to verify that missing non-required secrets won't throw any errors
				Required: false,
				Default:  nil,
			},
		},
		"min": &model.VariableDefinition{
			Name: "min",
		},
		"desc": &model.VariableDefinition{
			Name: "desc",
			CVOptions: model.CVOptions{
				Description: "<<<a description>>>",
				Example:     "Use this",
			},
		},
		"valued": &model.VariableDefinition{
			Name: "valued",
			CVOptions: model.CVOptions{
				Description: "<<<invaluable>>>",
				Default:     "you are very valued indeed",
			},
		},
		"structured": &model.VariableDefinition{
			Name: "structured",
			CVOptions: model.CVOptions{
				Description: "<<<non-scalar>>>",
				Example:     "use: \"this\"\n",
				Default:     map[string]string{"non": "scalar"},
			},
		},
		"const": &model.VariableDefinition{
			Name: "const",
			CVOptions: model.CVOptions{
				Description: "<<<don't change>>>",
				Default:     "rock solid",
				Required:    true,
				Immutable:   true,
			},
		},
		"genie": &model.VariableDefinition{
			Name: "genie",
			Type: "password",
			CVOptions: model.CVOptions{
				Description: "<<<here is jeannie>>>",
			},
		},
		"guinevere": &model.VariableDefinition{
			// excluded from __helm__ output
			Name: "guinevere",
			Type: "password",
			CVOptions: model.CVOptions{
				Description: "<<<helm hidden>>>",
				Immutable:   true,
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

	renderedYAML, err := RenderNode(secret, nil)
	if !assert.NoError(err) {
		return
	}

	varStructuredJSON, _ := json.Marshal(testCV["structured"].CVOptions.Default)

	varConstB64 := RenderEncodeBase64(testCV["const"].CVOptions.Default.(string))
	varValuedB64 := RenderEncodeBase64(testCV["valued"].CVOptions.Default.(string))
	varStructuredB64 := RenderEncodeBase64(string(varStructuredJSON))

	// Check the comments, and also that they are associated with
	// the correct variables.

	asString := string(renderedYAML)

	assert.Contains(asString, fmt.Sprintf("# <<<don't change>>>\n  const: %q", varConstB64))
	assert.Contains(asString, "# <<<a description>>>\n  # Example: \"Use this\"\n  desc: \"\"")
	assert.Contains(asString, fmt.Sprintf("# <<<non-scalar>>>\n  # Example:\n  #   use: \"this\"\n  structured: %q", varStructuredB64))
	assert.Contains(asString, "\n  min: \"\"")
	assert.Contains(asString, fmt.Sprintf("# <<<invaluable>>>\n  valued: %q", varValuedB64))
	assert.Contains(asString, "# <<<here is jeannie>>>\n  genie: \"\"")
	assert.Contains(asString, "# <<<helm hidden>>>\n  guinevere: \"\"")

	actual, err := RoundtripKube(secret)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, fmt.Sprintf(`---
		apiVersion: "v1"
		data:
			const: %q
			desc: ""
			genie: ""
			guinevere: ""
			min: ""
			valued: %q
			structured: %q
			optional: ""
		kind: "Secret"
		metadata:
			name: "secrets"
			labels:
				skiff-role-name: "secrets"
	`, varConstB64, varValuedB64, varStructuredB64), actual)
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

		_, err := RenderNode(secret, nil)
		assert.EqualError(err,
			`template: :6:237: executing "" at <fail "secrets.const ...>: error calling fail: secrets.const has not been set`)
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

		_, err := RenderNode(secret, config)
		assert.EqualError(err,
			`template: :6:237: executing "" at <fail "secrets.const ...>: error calling fail: secrets.const has not been set`)
	})

	t.Run("Present", func(t *testing.T) {
		t.Parallel()
		varConst := "cannot change"
		varDesc := "self-explanatory"
		varMin := "rock-bottom"
		varValued := "sky high"
		varStructured := map[string]string{"key": "value"}
		varGenie := "djinn"

		varStructuredJSON, _ := json.Marshal(varStructured)

		varConstB64 := RenderEncodeBase64(varConst)
		varDescB64 := RenderEncodeBase64(varDesc)
		varMinB64 := RenderEncodeBase64(varMin)
		varValuedB64 := RenderEncodeBase64(varValued)
		varStructuredB64 := RenderEncodeBase64(string(varStructuredJSON))
		varGenieB64 := RenderEncodeBase64(varGenie)

		config := map[string]interface{}{
			// "Values.secrets.optional" is intentionally not defined
			"Values.secrets.const":      varConst,
			"Values.secrets.desc":       varDesc,
			"Values.secrets.min":        varMin,
			"Values.secrets.valued":     varValued,
			"Values.secrets.structured": varStructured,
			"Values.secrets.genie":      varGenie,
		}

		renderedYAML, err := RenderNode(secret, config)
		if !assert.NoError(err) {
			return
		}

		// Check the comments, and also that they are associated with
		// the correct variables.

		asString := string(renderedYAML)

		assert.Contains(asString, "# <<<don't change>>>\n  # This value is")
		assert.Contains(asString, fmt.Sprintf("# This value is immutable and must not be changed once set.\n  const: %q", varConstB64))
		assert.Contains(asString, fmt.Sprintf("# <<<a description>>>\n  # Example: \"Use this\"\n  desc: %q", varDescB64))
		assert.Contains(asString, fmt.Sprintf("# <<<non-scalar>>>\n  # Example:\n  #   use: \"this\"\n  structured: %q", varStructuredB64))
		assert.Contains(asString, fmt.Sprintf("\n  min: %q", varMinB64))
		assert.Contains(asString, fmt.Sprintf("# <<<invaluable>>>\n  valued: %q", varValuedB64))
		assert.Contains(asString, "# <<<here is jeannie>>>\n  # This value uses ")
		assert.Contains(asString, fmt.Sprintf("# This value uses a generated default.\n  genie: %q", varGenieB64))

		// And check overall structure

		actual, err := RoundtripNode(secret, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, fmt.Sprintf(`---
			apiVersion: "v1"
			data:
				const: %q
				desc: %q
				min: %q
				valued: %q
				structured: %q
				genie: %q
				optional: ""
			kind: "Secret"
			metadata:
				name: "secrets"
				labels:
					skiff-role-name: "secrets"
		`, varConstB64, varDescB64, varMinB64, varValuedB64, varStructuredB64, varGenieB64), actual)
	})
}
