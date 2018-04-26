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

	secretYAML, err := testhelpers.RenderNode(secret, nil)
	if !assert.NoError(err) {
		return
	}

	expectedAccountYAML := `---
apiVersion: "v1"
data: {}
kind: "Secret"
metadata:
  name: "secrets"
  labels:
    skiff-role-name: "secrets"
`
	assert.Equal(expectedAccountYAML, string(secretYAML))
}

func TestMakeSecretsEmptyForHelm(t *testing.T) {
	assert := assert.New(t)

	secret, err := MakeSecrets(model.CVMap{}, ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}

	secretYAML, err := testhelpers.RenderNode(secret, nil)
	if !assert.NoError(err) {
		return
	}

	expectedAccountYAML := `---
apiVersion: "v1"
data: {}
kind: "Secret"
metadata:
  name: "secrets"
  labels:
    skiff-role-name: "secrets"
`
	assert.Equal(expectedAccountYAML, string(secretYAML))
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

	secretYAML, err := testhelpers.RenderNode(secret, nil)
	// config - helm only
	if !assert.NoError(err) {
		return
	}

	expectedAccountYAML := `---
apiVersion: "v1"
data:
  # <<<don't change>>>
  const: "cm9jayBzb2xpZA=="

  # <<<a description>>>
  desc: ""

  # <<<here is jeannie>>>
  genie: ""

  # <<<helm hidden>>>
  guinevere: ""

  min: ""

  # <<<invaluable>>>
  valued: "eW91IGFyZSB2ZXJ5IHZhbHVlZCBpbmRlZWQ="

kind: "Secret"
metadata:
  name: "secrets"
  labels:
    skiff-role-name: "secrets"
`
	assert.Equal(expectedAccountYAML, string(secretYAML))
}

func TestMakeSecretsForHelmWithDefaults(t *testing.T) {
	assert := assert.New(t)

	secret, err := MakeSecrets(testCVMap(), ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}

	_, err = testhelpers.RenderNode(secret, nil)
	// config - helm only
	// Render with defaults (is expected to) fail(s) due to a
	// number of guards (secrets.FOO, FOO a variable) not having a
	// proper (non-nil) value.
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

	secretYAML, err := testhelpers.RenderNode(secret, config)
	// config - helm only
	if !assert.NoError(err) {
		return
	}

	expectedAccountYAML := `---
apiVersion: "v1"
data:
  # <<<don't change>>>
  # This value is immutable and must not be changed once set.
  const: "Y2Fubm90IGNoYW5nZQ=="

  # <<<a description>>>
  desc: "c2VsZi1leHBsYW5hdG9yeQ=="

  min: "cm9jay1ib3R0b20="

  # <<<invaluable>>>
  valued: "c2t5IGhpZ2g="

  # <<<here is jeannie>>>
  # This value uses a generated default.
  genie: "ZGppbm4="

kind: "Secret"
metadata:
  name: "secrets"
  labels:
    skiff-role-name: "secrets"
`

	assert.Equal(expectedAccountYAML, string(secretYAML))
}
