package kube

import (
	"fmt"

	"code.cloudfoundry.org/fissile/helm"
)

// MakeBoshDeploymentManifestSecret generates a template for a secret that holds the content of a BOSH deployment manifest
func MakeBoshDeploymentManifestSecret(settings ExportSettings) (helm.Node, error) {
	value := ""
	if settings.CreateHelmChart {
		value = "{{ .Values.bosh | toYaml | b64enc }}"
	}

	cb := NewConfigBuilder().
		SetSettings(&settings).
		SetAPIVersion("v1").
		SetKind("Secret").
		SetName("deployment-manifest")
	secret, err := cb.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build a new kube config: %v", err)
	}

	data := helm.NewMapping("deployment-manifest", value)
	secret.Add("data", data)
	secret.Add("type", "Opaque")

	return secret, nil
}
