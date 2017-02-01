package kube

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/hpcloud/fissile/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/runtime"
)

func isYAMLSubset(assert *assert.Assertions, expected, actual interface{}, prefix []string) bool {
	yamlPath := strings.Join(prefix, ".")
	actualType := reflect.TypeOf(actual)
	if expectedMap, ok := expected.(map[interface{}]interface{}); ok {
		actualMap, ok := actual.(map[interface{}]interface{})
		if !assert.True(ok, "expected YAML path %s to be a map, but is actually %s", yamlPath, actualType) {
			return false
		}
		success := true
		for key, value := range expectedMap {
			thisPrefix := append(prefix, fmt.Sprintf("%s", key))
			yamlPath = strings.Join(prefix, ".")
			if assert.Contains(actualMap, key, fmt.Sprintf("missing key %s in YAML path %s", key, yamlPath)) {
				if !isYAMLSubset(assert, value, actualMap[key], thisPrefix) {
					success = false
				}
			}
		}
		return success
	}
	if expectedSlice, ok := expected.([]interface{}); ok {
		actualSlice, ok := actual.([]interface{})
		if !assert.True(ok, "expected YAML path %s to be a slice, but is actually %s", yamlPath, actualType) {
			return false
		}
		if !assert.Len(actualSlice, len(expectedSlice), "expected slice at YAML path %s to have correct length", yamlPath) {
			return false
		}
		success := true
		for i := range expectedSlice {
			if !isYAMLSubset(assert, expectedSlice[i], actualSlice[i], append(prefix, fmt.Sprintf("%d", i))) {
				success = false
			}
		}
		return success
	}
	return assert.Equal(expected, actual, "unexpected value at YAML path %s", yamlPath)
}

func TestStatefulSetPorts(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/exposed-ports.yml")
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return
	}
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release})
	if !assert.NoError(err) {
		return
	}

	var role *model.Role
	for _, r := range manifest.Roles {
		if r != nil {
			if r.Name == "myrole" {
				role = r
			}
		}
	}
	if !assert.NotNil(role, "Failed to find role in manifest") {
		return
	}
	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	statefulset, deps := NewStatefulSet(role)
	var endpointService, headlessService *apiv1.Service

	if assert.Len(deps.Items, 2, "Should have two services per stateful role") {
		for _, item := range deps.Items {
			svc := item.Object.(*apiv1.Service)
			if svc.Spec.ClusterIP == apiv1.ClusterIPNone {
				headlessService = svc
			} else {
				endpointService = svc
			}
		}
	}
	if assert.NotNil(endpointService, "endpoint service not found") {
		assert.Equal(role.Name, endpointService.ObjectMeta.Name, "unexpected endpoint service name")
	}
	if assert.NotNil(headlessService, "headless service not found") {
		assert.Equal(role.Name+"-pod", headlessService.ObjectMeta.Name, "unexpected headless service name")
	}

	objects := apiv1.List{
		Items: append(deps.Items,
			runtime.RawExtension{
				Object: statefulset,
			}),
	}
	yamlConfig, err := GetYamlConfig(&objects)
	if !assert.NoError(err) {
		return
	}
	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal([]byte(yamlConfig), &actual)) {
		return
	}
	expectedYAML := strings.Replace(`---
		items:
		-
			# This is the public service port
			metadata:
				name: myrole
			spec:
				ports:
				-
						name: http
						port: 80
						targetPort: http
				-
						name: https
						port: 443
						targetPort: https
				selector:
					skiff-role-name: myrole
				type: ClusterIP
		-
			# This is the per-pod naming port
			metadata:
				name: myrole-pod
			spec:
				ports:
				-
					name: http
					port: 80
					targetPort: 0
				-
					name: https
					port: 443
					targetPort: 0
				selector:
					skiff-role-name: myrole
				type: ClusterIP
				clusterIP: None
		-
			# This is the actual StatefulSet
			metadata:
				name: myrole
			spec:
				replicas: 1
				serviceName: myrole-pod
				template:
					metadata:
						labels:
							skiff-role-name: myrole
						name: myrole
					spec:
						containers:
						-
							name: myrole
							ports:
							-
								name: http
								containerPort: 8080
								hostPort: 80
							-
								name: https
								containerPort: 443
								hostPort: 443
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	_ = isYAMLSubset(assert, expected, actual, []string{})
}
