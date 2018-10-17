package kube

import "code.cloudfoundry.org/fissile/helm"

// MakeBoshDeploymentManifestSecret generates a template for a secret that holds the content of a BOSH deployment manifest
func MakeBoshDeploymentManifestSecret(settings ExportSettings) (helm.Node, error) {
	value := ""
	if settings.CreateHelmChart {
		value = "{{ .Values.bosh | toYaml | b64enc }}"
	}

	secret := newKubeConfig(settings, "v1", "Secret", "deployment-manifest", boshDeploymentManifestCondition(settings))

	data := helm.NewMapping("deployment-manifest", value)
	secret.Add("data", data)
	secret.Add("type", "Opaque")

	return secret, nil
}

// boshDeploymentManifestCondition creates a block condition checking for an embedded bosh deployment manifest
func boshDeploymentManifestCondition(settings ExportSettings) helm.NodeModifier {
	if settings.CreateHelmChart {
		return helm.Block("if .Values.bosh")
	}
	return nil
}
