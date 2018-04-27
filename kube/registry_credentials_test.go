package kube

import (
	"fmt"
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

	// Below we see how helm renders the registry secret when no
	// registry (host) is specified, with no user/pass either.
	//
	// dcfg :=
	// 	{%!q(<nil>):{"username":"","password":"","auth":"JSFzKDxuaWw+KTolIXMoPG5pbD4p"}}
	// auth :=
	// 	%!s(<nil>):%!s(<nil>)

	user := "" // default from nil
	pass := "" // ditto
	qnil := "%!q(<nil>)"
	snil := "%!s(<nil>)"

	auth64 := testhelpers.RenderEncodeBase64(fmt.Sprintf("%s:%s", snil, snil))
	// user, pass are nil, and helm renders that as snil.

	dcfg := testhelpers.RenderEncodeBase64(fmt.Sprintf(
		`{%s:{"username":%q,"password":%q,"auth":%q}}`,
		qnil, user, pass, auth64))
	// host is nil, and rendered as qnil by helm

	actual, err := testhelpers.RoundtripNode(registryCredentials, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, fmt.Sprintf(`---
		apiVersion: "v1"
		data:
			.dockercfg: %s
		kind: "Secret"
		metadata:
			name: "registry-credentials"
			labels:
				skiff-role-name: "registry-credentials"
		type: "kubernetes.io/dockercfg"
	`, dcfg), actual)
}

func TestMakeRegistryCredentialsForHelmWithUserChoice(t *testing.T) {
	assert := assert.New(t)

	registryCredentials, err := MakeRegistryCredentials(ExportSettings{
		CreateHelmChart: true,
	})

	if !assert.NoError(err) {
		return
	}

	// And a registry with user and pass.

	user := "the-user"
	pass := "the-password"
	host := "the-host"
	auth64 := testhelpers.RenderEncodeBase64(fmt.Sprintf("%s:%s", user, pass))
	dcfg := testhelpers.RenderEncodeBase64(fmt.Sprintf(
		`{%q:{"username":%q,"password":%q,"auth":%q}}`,
		host, user, pass, auth64))

	config := map[string]interface{}{
		"Values.kube.registry.hostname": host,
		"Values.kube.registry.username": user,
		"Values.kube.registry.password": pass,
	}

	actual, err := testhelpers.RoundtripNode(registryCredentials, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, fmt.Sprintf(`---
		apiVersion: "v1"
		data:
			.dockercfg: %s
		kind: "Secret"
		metadata:
			name: "registry-credentials"
			labels:
				skiff-role-name: "registry-credentials"
		type: "kubernetes.io/dockercfg"
	`, dcfg), actual)
}
