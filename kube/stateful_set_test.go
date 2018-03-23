package kube

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"
	"github.com/stretchr/testify/assert"

	yaml "gopkg.in/yaml.v2"
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
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release}, nil)
	if !assert.NoError(err) {
		return nil, nil
	}

	role := manifest.LookupRole("myrole")
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

	statefulset, deps, err := NewStatefulSet(role, ExportSettings{}, nil)
	if !assert.NoError(err) {
		return
	}

	var endpointService, headlessService, privateService helm.Node
	items := deps.Get("items").Values()
	if assert.Len(items, 3, "Should have three services per stateful role") {
		for _, item := range items {
			clusterIP := item.Get("spec", "clusterIP")
			if clusterIP != nil && clusterIP.String() == "None" {
				headlessService = item
			} else if item.Get("spec", "externalIPs") == nil {
				privateService = item
			} else {
				endpointService = item
			}
		}
	}
	if assert.NotNil(endpointService, "endpoint service not found") {
		assert.Equal(role.Name+"-public", endpointService.Get("metadata", "name").String(), "unexpected endpoint service name")
	}
	if assert.NotNil(headlessService, "headless service not found") {
		assert.Equal(role.Name+"-set", headlessService.Get("metadata", "name").String(), "unexpected headless service name")
	}
	if assert.NotNil(privateService, "private service not found") {
		assert.Equal(role.Name, privateService.Get("metadata", "name").String(), "unexpected private service name")
	}

	items = append(items, statefulset)
	objects := helm.NewMapping("items", helm.NewNode(items))
	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(objects); !assert.NoError(err) {
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
	testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestStatefulSetVolumes(t *testing.T) {
	assert := assert.New(t)

	manifest, role := statefulSetTestLoadManifest(assert, "volumes.yml")
	if manifest == nil || role == nil {
		return
	}

	statefulset, _, err := NewStatefulSet(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err) {
		return
	}

	yamlConfig := &bytes.Buffer{}
	err = helm.NewEncoder(yamlConfig).Encode(statefulset)
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
							name: host-volume
							mountPath: /sys/fs/cgroup
						-
							name: persistent-volume
							mountPath: /mnt/persistent
						-
							name: shared-volume
							mountPath: /mnt/shared
					volumes:
					-
						name: host-volume
						hostPath:
							path: /sys/fs/cgroup
							type: Directory
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
	testhelpers.IsYAMLSubset(assert, expected, actual)
}
