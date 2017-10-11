package kube

import (
	"github.com/SUSE/fissile/helm"
)

// MakeRegistryCredentials generates a template that contains Docker Registry credentials
func MakeRegistryCredentials(createHelmChart bool) (helm.Node, error) {

	value := ""
	if createHelmChart {
		value = "{{ printf \"{%s:{%s:%s,%s:%s,%s:%s}}\" (.Values.kube.registry.hostname | quote) (printf \"%s\" \"username\" | quote) (default \"\" .Values.kube.registry.username | quote) ( printf \"%s\" \"password\" | quote) (default \"\" .Values.kube.registry.password | quote) ( printf \"%s\" \"auth\" | quote) ( printf \"%s:%s\" .Values.kube.registry.username .Values.kube.registry.password | b64enc | quote ) | b64enc }}"
	}

	data := helm.NewMapping(".dockercfg", value)

	secret := newKubeConfig("v1", "Secret", "registry-credentials")
	secret.Add("data", data)
	secret.Add("type", "kubernetes.io/dockercfg")

	return secret.Sort(), nil
}
