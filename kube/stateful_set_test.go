package kube

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statefulSetTestLoadManifest(assert *assert.Assertions, manifestName string) (*model.RoleManifest, *model.InstanceGroup) {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/kube", manifestName)
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	manifest, err := model.LoadRoleManifest(manifestPath, model.LoadRoleManifestOptions{
		ReleasePaths: []string{releasePath},
		BOSHCacheDir: filepath.Join(workDir, "../test-assets/bosh-cache"),
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	if !assert.NoError(err) {
		return nil, nil
	}

	role := manifest.LookupInstanceGroup("myrole")
	if !assert.NotNil(role, "Failed to find role in manifest") {
		return nil, nil
	}
	return manifest, role
}

func TestStatefulSetPorts(t *testing.T) {
	manifest, role := statefulSetTestLoadManifest(assert.New(t), "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0]
	require.NotNil(t, portDef)

	statefulset, deps, err := NewStatefulSet(role, ExportSettings{}, nil)
	require.NoError(t, err)

	var endpointService, headlessService, privateService helm.Node
	items := deps.Get("items").Values()
	if assert.Len(t, items, 4, "Should have four services per stateful role") {
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
	if assert.NotNil(t, endpointService, "endpoint service not found") {
		assert.Equal(t, role.Name+"-tor-public", endpointService.Get("metadata", "name").String(), "unexpected endpoint service name")
	}
	if assert.NotNil(t, headlessService, "headless service not found") {
		assert.Equal(t, role.Name+"-tor-set", headlessService.Get("metadata", "name").String(), "unexpected headless service name")
	}
	if assert.NotNil(t, privateService, "private service not found") {
		assert.Equal(t, role.Name+"-tor", privateService.Get("metadata", "name").String(), "unexpected private service name")
	}

	items = append(items, statefulset)
	objects := helm.NewMapping("items", helm.NewNode(items))

	actual, err := RoundtripKube(objects)
	require.NoError(t, err)

	expected := `---
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
					app.kubernetes.io/component: myrole
				clusterIP: None
		-
			# This is the per-pod naming port
			metadata:
				name: myrole-tor-set
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
					app.kubernetes.io/component: myrole
				clusterIP: None
		-
			# This is the private service port
			metadata:
				name: myrole-tor
			spec:
				ports:
				-
						name: http
						port: 80
						targetPort: 8080
				-
						name: https
						port: 443
						targetPort: 443
				selector:
					app.kubernetes.io/component: myrole
		-
			# This is the public service port
			metadata:
				name: myrole-tor-public
			spec:
				ports:
				-
						name: https
						port: 443
						targetPort: 443
				selector:
					app.kubernetes.io/component: myrole
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
							app.kubernetes.io/component: myrole
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
	`
	testhelpers.IsYAMLSubsetString(assert.New(t), expected, actual)
}

// TestStatefulSetServices checks that the services associated with a service
// are created correctly.
func TestStatefulSetServices(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"headless", "headed"} {
		func(variant string) {
			t.Run(variant, func(t *testing.T) {
				t.Parallel()
				manifestName := "service-" + variant + ".yml"
				manifest, role := statefulSetTestLoadManifest(assert.New(t), manifestName)
				require.NotNil(t, manifest)
				require.NotNil(t, role)
				require.NotEmpty(t, role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0], "No exposed ports loaded")

				statefulset, deps, err := NewStatefulSet(role, ExportSettings{}, nil)
				require.NoError(t, err)
				assert.NotNil(t, statefulset)
				assert.NotNil(t, deps)
				items := deps.Get("items").Values()

				var genericService, headlessService, internalService, publicService helm.Node
				for _, item := range items {
					switch item.Get("metadata").Get("name").String() {
					case "myrole-set":
						assert.Nil(t, genericService, "Multiple generic services found")
						genericService = item
					case "myrole-tor-set":
						assert.Nil(t, headlessService, "Multiple headless services found")
						headlessService = item
					case "myrole-tor":
						assert.Nil(t, internalService, "Multiple internal services found")
						internalService = item
					case "myrole-tor-public":
						assert.Nil(t, publicService, "Multiple public services found")
						publicService = item
					default:
						assert.Fail(t, "Found unexpected service: \n%s", item.String())
					}
				}
				for _, style := range []string{"kube", "helm"} {
					t.Run(style, func(t *testing.T) {
						if assert.NotNil(t, headlessService, "Headless service not found") {
							var actual interface{}
							var err error
							switch style {
							case "helm":
								actual, err = RoundtripNode(headlessService, nil)
							case "kube":
								actual, err = RoundtripKube(headlessService)
							default:
								panic("Unexpected style " + style)
							}
							require.NoError(t, err)
							testhelpers.IsYAMLEqualString(assert.New(t), `---
							apiVersion: v1
							kind: Service
							metadata:
								name: myrole-tor-set
								labels:
									app.kubernetes.io/component: myrole-tor-set
							spec:
								clusterIP: None
								ports:
								-
									name: http
									port: 80
									protocol: TCP
									targetPort: 0
								-
									name: https
									port: 443
									protocol: TCP
									targetPort: 0
								selector:
									app.kubernetes.io/component: myrole
							`, actual)
						}
						if assert.NotNil(t, genericService, "Generic instance group service not found") {
							var actual interface{}
							var err error
							switch style {
							case "helm":
								actual, err = RoundtripNode(genericService, nil)
							case "kube":
								actual, err = RoundtripKube(genericService)
							default:
								panic("Unexpected style " + style)
							}
							require.NoError(t, err)
							testhelpers.IsYAMLEqualString(assert.New(t), `---
							apiVersion: v1
							kind: Service
							metadata:
								name: myrole-set
								labels:
									app.kubernetes.io/component: myrole-set
							spec:
								clusterIP: None
								ports:
								-
									name: http
									port: 80
									protocol: TCP
									targetPort: 0
								-
									name: https
									port: 443
									protocol: TCP
									targetPort: 0
								selector:
									app.kubernetes.io/component: myrole
							`, actual)
						}
						if assert.NotNil(t, publicService, "Public service not found") {
							var actual interface{}
							var err error
							switch style {
							case "helm":
								actual, err = RoundtripNode(publicService, nil)
							case "kube":
								actual, err = RoundtripKube(publicService)
							default:
								panic("Unexpected style " + style)
							}
							require.NoError(t, err)
							testhelpers.IsYAMLEqualString(assert.New(t), `---
							apiVersion: v1
							kind: Service
							metadata:
								name: myrole-tor-public
								labels:
									app.kubernetes.io/component: myrole-tor-public
							spec:
								externalIPs: [ 192.168.77.77 ]
								ports:
								-
									name: https
									port: 443
									protocol: TCP
									targetPort: 443
								selector:
									app.kubernetes.io/component: myrole
							`, actual)
						}
						if assert.NotNil(t, internalService, "Internal service not found") {
							var actual interface{}
							var err error
							switch style {
							case "helm":
								actual, err = RoundtripNode(internalService, nil)
							case "kube":
								actual, err = RoundtripKube(internalService)
							default:
								panic("Unexpected style " + style)
							}
							require.NoError(t, err)
							testhelpers.IsYAMLEqualString(assert.New(t), `---
							apiVersion: v1
							kind: Service
							metadata:
								name: myrole-tor
								labels:
									app.kubernetes.io/component: myrole-tor
							spec:
								ports:
								-
									name: http
									port: 80
									protocol: TCP
									targetPort: 8080
								-
									name: https
									port: 443
									protocol: TCP
									targetPort: 443
								selector:
									app.kubernetes.io/component: myrole
							`, actual)
						}
					})
				}
			})
		}(variant)
	}
}

// TestStatefulSetStart checks that roles with the `sequential-startup` tag will
// be of OrderedReady podManagementPolicy; and that roles without have Parallel.
func TestStatefulSetStartupPolicy(t *testing.T) {
	t.Parallel()
	_, roleTemplate := statefulSetTestLoadManifest(assert.New(t), "volumes.yml")
	require.NotNil(t, roleTemplate)
	testCases := map[string][]model.RoleTag{
		"OrderedReady": []model.RoleTag{model.RoleTagSequentialStartup},
		"Parallel":     []model.RoleTag{},
	}
	for policy, tags := range testCases {
		func(policy string, tags []model.RoleTag) {
			t.Run(policy, func(t *testing.T) {
				t.Parallel()
				role := *roleTemplate
				role.Tags = tags

				t.Run("kube", func(t *testing.T) {
					t.Parallel()
					statefulset, _, err := NewStatefulSet(&role, ExportSettings{
						Opinions: model.NewEmptyOpinions(),
					}, nil)
					require.NoError(t, err)
					actual, err := RoundtripKube(statefulset)
					require.NoError(t, err)
					expected := `---
					spec:
						podManagementPolicy: %s
					`
					testhelpers.IsYAMLSubsetString(assert.New(t), fmt.Sprintf(expected, policy), actual)
				})

				t.Run("helm", func(t *testing.T) {
					t.Parallel()
					statefulset, _, err := NewStatefulSet(&role, ExportSettings{
						Opinions:        model.NewEmptyOpinions(),
						CreateHelmChart: true,
					}, nil)
					require.NoError(t, err)
					actual, err := RoundtripNode(statefulset, map[string]interface{}{
						"Values.sizing.myrole.count":                        "1",
						"Values.sizing.myrole.affinity":                     map[string]interface{}{},
						"Values.sizing.myrole.capabilities":                 []string{},
						"Values.sizing.myrole.disk_sizes.persistent_volume": 1,
					})
					require.NoError(t, err)
					expected := `---
					spec:
						podManagementPolicy: %s
					`
					testhelpers.IsYAMLSubsetString(assert.New(t), fmt.Sprintf(expected, policy), actual)
				})
			})
		}(policy, tags)
	}
}

func TestStatefulSetVolumesKube(t *testing.T) {
	t.Parallel()
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

	actual, err := RoundtripKube(statefulset)
	if !assert.NoError(err) {
		return
	}

	expected := `---
		metadata:
			name: myrole
		spec:
			replicas: 1
			serviceName: myrole-set
			template:
				metadata:
					labels:
						app.kubernetes.io/component: myrole
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
						-
							name: deployment-manifest
							mountPath: /opt/fissile/config

					volumes:
					-
						name: host-volume
						hostPath:
							path: /sys/fs/cgroup
					-
						name: deployment-manifest
						secret:
							secretName: deployment-manifest
							items:
							-	key: deployment-manifest
								path: deployment-manifest.yml
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
	`
	testhelpers.IsYAMLSubsetString(assert, expected, actual)
}

func TestStatefulSetVolumesWithAnnotationKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := statefulSetTestLoadManifest(assert, "volumes-with-annotation.yml")
	if manifest == nil || role == nil {
		return
	}

	statefulset, _, err := NewStatefulSet(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err) {
		return
	}

	actual, err := RoundtripKube(statefulset)
	if !assert.NoError(err) {
		return
	}

	expected := `---
		metadata:
			name: myrole
		spec:
			replicas: 1
			serviceName: myrole-set
			template:
				metadata:
					labels:
						app.kubernetes.io/component: myrole
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
						-
							name: host-volume
							mountPath: /sys/fs/cgroup
						-
							name: deployment-manifest
							mountPath: /opt/fissile/config
					volumes:
					-
						name: host-volume
						hostPath:
							path: /sys/fs/cgroup
					-
						name: deployment-manifest
						secret:
							secretName: deployment-manifest
							items:
							-	key: deployment-manifest
								path: deployment-manifest.yml
			volumeClaimTemplates:
				-
					metadata:
						annotations:
							volume.beta.kubernetes.io/storage-class: a-company-file-gold
							volume.beta.kubernetes.io/storage-provisioner: a-company.io/storage-provisioner
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
							volume.beta.kubernetes.io/storage-provisioner: a-company.io/storage-provisioner
						name: shared-volume
					spec:
						accessModes: [ReadWriteMany]
						resources:
							requests:
								storage: 40G
	`
	testhelpers.IsYAMLSubsetString(assert, expected, actual)
}

func TestStatefulSetVolumesHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := statefulSetTestLoadManifest(assert, "volumes.yml")
	if manifest == nil || role == nil {
		return
	}

	statefulset, _, err := NewStatefulSet(role, ExportSettings{
		Opinions:        model.NewEmptyOpinions(),
		CreateHelmChart: true,
	}, nil)
	if !assert.NoError(err) {
		return
	}

	config := map[string]interface{}{
		"Values.env.ALL_VAR":                                "",
		"Values.kube.hostpath_available":                    true,
		"Values.kube.registry.hostname":                     "",
		"Values.kube.storage_class.persistent":              "persistent",
		"Values.kube.storage_class.shared":                  "shared",
		"Values.sizing.myrole.affinity":                     map[string]interface{}{},
		"Values.sizing.myrole.capabilities":                 []interface{}{},
		"Values.sizing.myrole.count":                        "1",
		"Values.sizing.myrole.disk_sizes.persistent_volume": "5",
		"Values.sizing.myrole.disk_sizes.shared_volume":     "40",
	}

	actual, err := RoundtripNode(statefulset, config)
	if !assert.NoError(err) {
		return
	}

	expected := `---
		metadata:
			name: myrole
		spec:
			replicas: 1
			serviceName: myrole-set
			template:
				metadata:
					labels:
						app.kubernetes.io/component: myrole
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
						-
							mountPath: /opt/fissile/config
							name: deployment-manifest
							readOnly: true
					volumes:
					-
						name: host-volume
						hostPath:
							path: /sys/fs/cgroup
							type: Directory
					-
						name: deployment-manifest
						secret:
							items:
							-	key: deployment-manifest
								path: deployment-manifest.yml
							secretName: deployment-manifest
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
	`
	testhelpers.IsYAMLSubsetString(assert, expected, actual)

	// Check that not having hostpath disables the hostpath volume
	overrides := map[string]interface{}{
		"Values.env.ALL_VAR":                                "",
		"Values.kube.hostpath_available":                    false,
		"Values.kube.registry.hostname":                     "",
		"Values.kube.storage_class.persistent":              "persistent",
		"Values.sizing.myrole.affinity":                     map[string]interface{}{},
		"Values.sizing.myrole.capabilities":                 []interface{}{},
		"Values.sizing.myrole.count":                        "1",
		"Values.sizing.myrole.disk_sizes.persistent_volume": "5",
	}
	actual, err = RoundtripNode(statefulset, overrides)
	if !assert.NoError(err) {
		return
	}
	volumes := actual
	for _, k := range []string{"spec", "template", "spec", "volumes"} {
		volumes = volumes.(map[interface{}]interface{})[k]
	}
	assert.Len(volumes, 1, "Hostpath volumes should not be available")
	assert.Equal("deployment-manifest", volumes.([]interface{})[0].(map[interface{}]interface{})["name"])
}

func TestStatefulSetEmptyDirVolumesKube(t *testing.T) {
	assert := assert.New(t)

	manifest, role := statefulSetTestLoadManifest(assert, "colocated-containers-with-stateful-set-and-empty-dir.yml")
	if manifest == nil || role == nil {
		return
	}

	statefulset, _, err := NewStatefulSet(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err) {
		return
	}

	actual, err := RoundtripKube(statefulset)
	if !assert.NoError(err) {
		return
	}

	expected := `---
		metadata:
			name: myrole
		spec:
			replicas: 1
			serviceName: myrole-set
			template:
				metadata:
					labels:
						app.kubernetes.io/component: myrole
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
							name: shared-data
							mountPath: /mnt/shared-data
						-
							name: deployment-manifest
							mountPath: /opt/fissile/config
					-
						name: colocated
						volumeMounts:
						-
							name: shared-data
							mountPath: /mnt/shared-data
						-
							name: deployment-manifest
							mountPath: /opt/fissile/config
					volumes:
					-
						name: host-volume
						hostPath:
							path: /sys/fs/cgroup
					-
						name: shared-data
						emptyDir: {}
					-
						name: deployment-manifest
						secret:
							secretName: deployment-manifest
							items:
							-	key: deployment-manifest
								path: deployment-manifest.yml
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
	`
	testhelpers.IsYAMLSubsetString(assert, expected, actual)
}
