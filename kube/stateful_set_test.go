package kube

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"
	"github.com/SUSE/fissile/util"
	"github.com/stretchr/testify/assert"

	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
)

func statefulSetTestLoadManifest(assert *assert.Assertions, manifestName string) (*model.RoleManifest, *model.Role) {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests", manifestName)
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return nil, nil
	}
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release})
	if !assert.NoError(err) {
		return nil, nil
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
		return nil, nil
	}

	return manifest, role
}

func TestStatefulSetPorts(t *testing.T) {
	assert := assert.New(t)

	manifest, role := statefulSetTestLoadManifest(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	statefulset, deps, err := NewStatefulSet(role, &ExportSettings{}, util.VerbosityDefault)
	if !assert.NoError(err) {
		return
	}
	var endpointService, headlessService *apiv1.Service

	if assert.Len(deps.Items, 3, "Should have three services per stateful role") {
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
		assert.Equal(role.Name+"-public", endpointService.ObjectMeta.Name, "unexpected endpoint service name")
	}
	if assert.NotNil(headlessService, "headless service not found") {
		assert.Equal(role.Name+"-set", headlessService.ObjectMeta.Name, "unexpected headless service name")
	}

	objects := apiv1.List{
		Items: append(deps.Items,
			runtime.RawExtension{
				Object: statefulset,
			}),
	}
	yamlConfig := bytes.Buffer{}
	if err := WriteYamlConfig(&objects, &yamlConfig); !assert.NoError(err) {
		return
	}
	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}
	expectedYAML := strings.Replace(`---
		items:
		-
			# This is the per-pod naming port
			metadata:
				name: myrole-set
			spec:
				ports:
				-
					name: http
					port: 80
					# targetPort must be undefined for headless services
					targetPort: 0
				-
					name: https
					port: 443
					# targetPort must be undefined for headless services
					targetPort: 0
				selector:
					skiff-role-name: myrole
				type: ClusterIP
				clusterIP: None
		-
			# This is the private service port
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
			# This is the public service port
			metadata:
				name: myrole-public
			spec:
				ports:
				-
						name: https
						port: 443
						targetPort: https
				selector:
					skiff-role-name: myrole
				type: ClusterIP
		-
			# This is the actual StatefulSet
			metadata:
				name: myrole
			spec:
				replicas: 1
				serviceName: myrole-set
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
							-
								name: https
								containerPort: 443
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	_ = testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestStatefulSetVolumes(t *testing.T) {
	assert := assert.New(t)

	manifest, role := statefulSetTestLoadManifest(assert, "volumes.yml")
	if manifest == nil || role == nil {
		return
	}

	statefulset, _, err := NewStatefulSet(role, &ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, util.VerbosityDefault)
	if !assert.NoError(err) {
		return
	}

	yamlConfig := bytes.Buffer{}
	err = WriteYamlConfig(statefulset, &yamlConfig)
	if !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}
	expectedYAML := strings.Replace(`---
	metadata:
		name: myrole
	spec:
		replicas: 1
		serviceName: myrole-set
		template:
			metadata:
				labels:
					skiff-role-name: myrole
				name: myrole
			spec:
				containers:
				-
					name: myrole
					volumeMounts:
					-
						name: persistent-volume
						mountPath: /mnt/persistent
					-
						name: shared-volume
						mountPath: /mnt/shared
		volumeClaimTemplates:
			-
				metadata:
					annotations:
						volume.beta.kubernetes.io/storage-class: persistent
					name: persistent-volume
				spec:
					accessModes: [ReadWriteOnce]
					resources:
						requests:
							storage: 5G
			-
				metadata:
					annotations:
						volume.beta.kubernetes.io/storage-class: shared
					name: shared-volume
				spec:
					accessModes: [ReadWriteMany]
					resources:
						requests:
							storage: 40G
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	_ = testhelpers.IsYAMLSubset(assert, expected, actual)
}
