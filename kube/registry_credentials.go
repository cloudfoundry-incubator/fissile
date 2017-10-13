package kube

import (
	"github.com/SUSE/fissile/helm"
)

// MakeRegistryCredentials generates a template that contains Docker Registry credentials
func MakeRegistryCredentials(createHelmChart bool) (helm.Node, error) {

	value := ""
	if createHelmChart {
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
			`"username" (default "" .Values.kube.registry.username) ` +
			`"password" (default "" .Values.kube.registry.password) ` +
			`"auth" (printf "%s:%s" .Values.kube.registry.username .Values.kube.registry.password | b64enc ) ` +
			`| b64enc }}`
	}

	data := helm.NewMapping(".dockercfg", value)

	secret := newKubeConfig("v1", "Secret", "registry-credentials")
	secret.Add("data", data)
	secret.Add("type", "kubernetes.io/dockercfg")

	return secret.Sort(), nil
}
