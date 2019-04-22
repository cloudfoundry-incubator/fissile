package kube

import (
	"fmt"

	"code.cloudfoundry.org/fissile/helm"
)

// MakeRegistryCredentials generates a template that contains Docker Registry credentials
func MakeRegistryCredentials(settings ExportSettings) (helm.Node, error) {

	value := ""
	if settings.CreateHelmChart {
		// Registry secrets are in json format:
		// {
		//  "docker.io": {
		//      "username": "foo",
		//      "password": "bar",
		//      "auth": "Zm9vOmJhcg=="
		//   }
		// }
		//
		// where "auth" is a base64 encoded "username:password"

		value = `{{ printf "{%q:{%q:%q,%q:%q,%q:%q}}" ` +
			`.Values.kube.registry.hostname ` +
			`"username" .Values.kube.registry.username ` +
			`"password" .Values.kube.registry.password ` +
			`"auth" (printf "%s:%s" .Values.kube.registry.username .Values.kube.registry.password | b64enc) ` +
			`| b64enc }}`
	}

	data := helm.NewMapping(".dockercfg", value)

	cb := NewConfigBuilder().
		SetSettings(&settings).
		SetAPIVersion("v1").
		SetKind("Secret").
		SetName("registry-credentials")
	if settings.CreateHelmChart {
		cb.AddModifier(helm.Block(`if ne .Values.kube.registry.username ""`))
	}
	secret, err := cb.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build a new kube config: %v", err)
	}
	secret.Add("data", data)
	secret.Add("type", "kubernetes.io/dockercfg")

	return secret.Sort(), nil
}
