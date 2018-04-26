package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"
)

func TestMakeSecretsEmptyForKube(t *testing.T) {
	assert := assert.New(t)

	secret, err := MakeSecrets(model.CVMap{}, ExportSettings{})
	if !assert.NoError(err) {
		return
	}

	actual, err := testhelpers.RoundtripNode(secret, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data: {}
		kind: "Secret"
		metadata:
			name: "secrets"
			labels:
				skiff-role-name: "secrets"
	`, actual)
}

func TestMakeSecretsEmptyForHelm(t *testing.T) {
	assert := assert.New(t)

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
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data: {}
		kind: "Secret"
		metadata:
			name: "secrets"
			labels:
				skiff-role-name: "secrets"
	`, actual)
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
			// eW91IGFyZSB2ZXJ5IHZhbHVlZCBpbmRlZWQ=
		},
		"const": &model.ConfigurationVariable{
			Name:        "const",
			Description: "<<<don't change>>>",
			Default:     "rock solid",
			// cm9jayBzb2xpZA==
			Immutable: true,
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

func TestMakeSecretsForKube(t *testing.T) {
	assert := assert.New(t)

	secret, err := MakeSecrets(testCVMap(), ExportSettings{})
	if !assert.NoError(err) {
		return
	}

	actual, err := testhelpers.RenderNode(secret, nil)
	if !assert.NoError(err) {
		return
	}

	// Check the comments, and also that they are associated with
	// the correct variables.

	astring := string(actual)

	assert.Contains(astring, "# <<<don't change>>>\n  const: \"cm9jayBzb2xpZA==\"")
	assert.Contains(astring, "# <<<a description>>>\n  desc: \"\"")
	assert.Contains(astring, "\n  min: \"\"")
	assert.Contains(astring, "# <<<invaluable>>>\n  valued: \"eW91IGFyZSB2ZXJ5IHZhbHVlZCBpbmRlZWQ=\"")
	assert.Contains(astring, "# <<<here is jeannie>>>\n  genie: \"\"")
	assert.Contains(astring, "# <<<helm hidden>>>\n  guinevere: \"\"")

	actualh, err := testhelpers.RoundtripNode(secret, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data:
			const: "cm9jayBzb2xpZA=="
			desc: ""
			genie: ""
			guinevere: ""
			min: ""
			valued: "eW91IGFyZSB2ZXJ5IHZhbHVlZCBpbmRlZWQ="
		kind: "Secret"
		metadata:
			name: "secrets"
			labels:
				skiff-role-name: "secrets"
	`, actualh)
}

func TestMakeSecretsForHelmWithDefaults(t *testing.T) {
	assert := assert.New(t)

	secret, err := MakeSecrets(testCVMap(), ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}

	// config - helm only
	// Render with defaults (is expected to) fail(s) due to a
	// number of guards (secrets.FOO, FOO a variable) not having a
	// proper (non-nil) value.

	_, err = testhelpers.RenderNode(secret, nil)
	assert.EqualError(err, `template: :6:12: executing "" at <required "secrets.co...>: error calling required: secrets.const has not been set`)
}

func TestMakeSecretsForHelmOk(t *testing.T) {
	assert := assert.New(t)

	secret, err := MakeSecrets(testCVMap(), ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}

	config := map[string]interface{}{
		"Values.secrets.const":  "cannot change",    // Y2Fubm90IGNoYW5nZQ==
		"Values.secrets.desc":   "self-explanatory", // c2VsZi1leHBsYW5hdG9yeQ==
		"Values.secrets.min":    "rock-bottom",      // cm9jay1ib3R0b20=
		"Values.secrets.valued": "sky high",         // c2t5IGhpZ2g=
		"Values.secrets.genie":  "djinn",            // ZGppbm4=
	}

	actual, err := testhelpers.RenderNode(secret, config)
	if !assert.NoError(err) {
		return
	}

	// Check the comments, and also that they are associated with
	// the correct variables.

	astring := string(actual)

	assert.Contains(astring, "# <<<don't change>>>\n  # This value is")
	assert.Contains(astring, "# This value is immutable and must not be changed once set.\n  const: \"Y2Fubm90IGNoYW5nZQ==\"")
	assert.Contains(astring, "# <<<a description>>>\n  desc: \"c2VsZi1leHBsYW5hdG9yeQ==\"")
	assert.Contains(astring, "\n  min: \"cm9jay1ib3R0b20=\"")
	assert.Contains(astring, "# <<<invaluable>>>\n  valued: \"c2t5IGhpZ2g=\"")
	assert.Contains(astring, "# <<<here is jeannie>>>\n  # This value uses ")
	assert.Contains(astring, "# This value uses a generated default.\n  genie: \"ZGppbm4=\"")

	// And check overall structure

	actualh, err := testhelpers.RoundtripNode(secret, config)
	if !assert.NoError(err) {
		return
	}

	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data:
			const: "Y2Fubm90IGNoYW5nZQ=="
			desc: "c2VsZi1leHBsYW5hdG9yeQ=="
			min: "cm9jay1ib3R0b20="
			valued: "c2t5IGhpZ2g="
			genie: "ZGppbm4="
		kind: "Secret"
		metadata:
			name: "secrets"
			labels:
				skiff-role-name: "secrets"
	`, actualh)
}
