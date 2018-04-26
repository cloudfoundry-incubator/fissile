package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SUSE/fissile/testhelpers"
)

func TestMakeRegistryCredentialsForKube(t *testing.T) {
	assert := assert.New(t)

	registryCredentials, err := MakeRegistryCredentials(ExportSettings{})

	if !assert.NoError(err) {
		return
	}

	actual, err := testhelpers.RoundtripNode(registryCredentials, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data:
			.dockercfg: ""
		kind: "Secret"
		metadata:
			name: "registry-credentials"
			labels:
				skiff-role-name: "registry-credentials"
		type: "kubernetes.io/dockercfg"
	`, actual)
}

func TestMakeRegistryCredentialsForHelmWithDefaults(t *testing.T) {
	assert := assert.New(t)

	registryCredentials, err := MakeRegistryCredentials(ExportSettings{
		CreateHelmChart: true,
	})

	if !assert.NoError(err) {
		return
	}

	// .Values.kube.registry.hostname
	// .Values.kube.registry.username
	// .Values.kube.registry.password
	// json, base64 encoded

	// Notes: The base64 decodes to
	// 	{%!q(<nil>):{"username":"","password":"","auth":"JSFzKDxuaWw+KTolIXMoPG5pbD4p"}}
	// and the auth value decodes to
	// 	%!s(<nil>):%!s(<nil>)

	actual, err := testhelpers.RoundtripNode(registryCredentials, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data:
			.dockercfg: eyUhcSg8bmlsPik6eyJ1c2VybmFtZSI6IiIsInBhc3N3b3JkIjoiIiwiYXV0aCI6IkpTRnpLRHh1YVd3K0tUb2xJWE1vUEc1cGJENHAifX0=
		kind: "Secret"
		metadata:
			name: "registry-credentials"
			labels:
				skiff-role-name: "registry-credentials"
		type: "kubernetes.io/dockercfg"
	`, actual)
}

func TestMakeRegistryCredentialsForHelmWithUserChoice(t *testing.T) {
	assert := assert.New(t)

	registryCredentials, err := MakeRegistryCredentials(ExportSettings{
		CreateHelmChart: true,
	})

	if !assert.NoError(err) {
		return
	}

	config := map[string]interface{}{
		"Values.kube.registry.hostname": "the-host",
		"Values.kube.registry.username": "the-user",
		"Values.kube.registry.password": "the-password",
	}

	// Notes: The base64 decodes to
	//	{"the-host":{"username":"the-user","password":"the-password","auth":"dGhlLXVzZXI6dGhlLXBhc3N3b3Jk"}}
	// and the auth value decodes to
	//	the-user:the-password

	actual, err := testhelpers.RoundtripNode(registryCredentials, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		data:
			.dockercfg: eyJ0aGUtaG9zdCI6eyJ1c2VybmFtZSI6InRoZS11c2VyIiwicGFzc3dvcmQiOiJ0aGUtcGFzc3dvcmQiLCJhdXRoIjoiZEdobExYVnpaWEk2ZEdobExYQmhjM04zYjNKayJ9fQ==
		kind: "Secret"
		metadata:
			name: "registry-credentials"
			labels:
				skiff-role-name: "registry-credentials"
		type: "kubernetes.io/dockercfg"
	`, actual)
}
