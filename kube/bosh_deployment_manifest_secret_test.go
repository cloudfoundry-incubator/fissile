package kube

import (
	b64 "encoding/base64"
	"testing"

	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestMakeBoshDeploymentManifestSecretKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifestSecret, err := MakeBoshDeploymentManifestSecret(ExportSettings{})

	if !assert.NoError(err) {
		return
	}

	actual, err := RoundtripKube(manifestSecret)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data:
		  deployment-manifest: ""
		kind: "Secret"
		metadata:
			name: "deployment-manifest"
			labels:
				app.kubernetes.io/component: "deployment-manifest"
		type: "Opaque"
	`, actual)
}

func TestMakeBoshDeploymentManifestSecretHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifestSecret, err := MakeBoshDeploymentManifestSecret(ExportSettings{
		CreateHelmChart: true,
	})
	if !assert.NoError(err) {
		return
	}

	config := map[string]interface{}{
		"Values.bosh.foo": "bar",
	}

	actual, err := RoundtripNode(manifestSecret, config)
	if !assert.NoError(err) {
		return
	}

	payload := b64.StdEncoding.EncodeToString([]byte("foo: bar"))

	testhelpers.IsYAMLEqualString(assert, `---
	apiVersion: "v1"
	data:
	  deployment-manifest: `+payload+`
	kind: "Secret"
	metadata:
		name: "deployment-manifest"
		labels:
			app.kubernetes.io/component: deployment-manifest
			app.kubernetes.io/instance: MyRelease
			app.kubernetes.io/managed-by: Tiller
			app.kubernetes.io/name: MyChart
			app.kubernetes.io/version: 1.22.333.4444
			helm.sh/chart: MyChart-42.1_foo
			skiff-role-name: "deployment-manifest"
	type: "Opaque"
	`, actual)
}
