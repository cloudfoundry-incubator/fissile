package kube

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/loader"
	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func podTemplateTestLoadRole(assert *assert.Assertions) *model.InstanceGroup {
	workDir, err := os.Getwd()
	if !assert.NoError(err) {
		return nil
	}

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/kube/volumes.yml")
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	manifest, err := loader.LoadRoleManifest(manifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{releasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	if !assert.NoError(err) {
		return nil
	}
	instanceGroup := manifest.LookupInstanceGroup("myrole")
	if !assert.NotNil(instanceGroup, "Failed to find role in manifest") {
		return nil
	}

	// Force a broadcast SECRET_VAR into the manifest, to see in all roles
	manifest.Variables = append(manifest.Variables,
		&model.VariableDefinition{
			Name: "SECRET_VAR",
			CVOptions: model.CVOptions{
				Type:     model.CVTypeUser,
				Secret:   true,
				Internal: true,
			},
		})
	return instanceGroup
}

type Sample struct {
	desc     string
	input    interface{}
	helm     map[string]interface{}
	expected string
	err      string
}

func (sample *Sample) check(t *testing.T, helmConfig helm.Node, err error) {
	t.Run(sample.desc, func(t *testing.T) {
		assert := assert.New(t)
		if sample.err != "" {
			assert.EqualError(err, sample.err, sample.desc)
			return
		}
		if !assert.NoError(err, sample.desc) {
			return
		}
		if sample.expected == "" {
			assert.Nil(helmConfig)
			return
		}
		actualYAML := &bytes.Buffer{}
		if helmConfig != nil && !assert.NoError(helm.NewEncoder(actualYAML).Encode(helmConfig)) {
			return
		}
		expectedYAML := strings.Replace(sample.expected, "-\t", "-   ", -1)
		expectedYAML = strings.Replace(expectedYAML, "\t", "    ", -1)

		var expected, actual interface{}
		if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
			assert.Fail(expectedYAML)
			return
		}
		if !assert.NoError(yaml.Unmarshal(actualYAML.Bytes(), &actual)) {
			assert.Fail(actualYAML.String())
			return
		}
		if !testhelpers.IsYAMLSubset(assert, expected, actual) {
			assert.Fail("Not a proper YAML subset", "*Actual*\n%s\n*Expected*\n%s\n", actualYAML.String(), expectedYAML)
		}
	})
}

func TestPodGetNonClaimVolumes(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	mounts := getNonClaimVolumes(role, ExportSettings{CreateHelmChart: true})
	assert.NotNil(mounts)

	config := map[string]interface{}{
		"Values.kube.hostpath_available": true,
		"Values.bosh.foo":                "bar",
	}
	actual, err := RoundtripNode(mounts, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "host-volume"
			hostPath:
				path: "/sys/fs/cgroup"
				type: "Directory"
		-	name: "deployment-manifest"
			secret:
				secretName: "deployment-manifest"
				items:
				-	key: deployment-manifest
					path: deployment-manifest.yml
	`, actual)
}

func TestPodGetVolumes(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	claims := getVolumeClaims(role, false)
	assert.Len(claims, 2, "expected two claims")

	var persistentVolume, sharedVolume *model.RoleRunVolume
	for _, volume := range role.Run.Volumes {
		switch volume.Type {
		case model.VolumeTypePersistent:
			persistentVolume = volume
		case model.VolumeTypeShared:
			sharedVolume = volume
		}
	}

	var persistentClaim, sharedClaim helm.Node
	for _, claim := range claims {
		switch claim.Get("metadata", "name").String() {
		case string(persistentVolume.Tag):
			persistentClaim = claim
		case string(sharedVolume.Tag):
			sharedClaim = claim
		default:
			assert.Fail("Got unexpected claim", "%s", claim)
		}
	}

	samples := []Sample{
		{
			desc:  "persistentClaim",
			input: persistentClaim,
			expected: `---
				metadata:
					name: persistent-volume
					annotations:
						volume.beta.kubernetes.io/storage-class: persistent
				spec:
					accessModes:
					-	ReadWriteOnce
					resources:
						requests:
							storage: 5G`,
		},
		{
			desc:  "sharedClaim",
			input: sharedClaim,
			expected: `---
				metadata:
					name: shared-volume
					annotations:
						volume.beta.kubernetes.io/storage-class: shared
				spec:
					accessModes:
					-	ReadWriteMany
					resources:
						requests:
							storage: 40G`,
		},
	}
	for _, sample := range samples {
		sample.check(t, sample.input.(helm.Node), nil)
	}
}

func TestPodGetVolumesHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	claims := getVolumeClaims(role, true)
	assert.Len(claims, 2, "expected two claims")

	var persistentVolume, sharedVolume *model.RoleRunVolume
	for _, volume := range role.Run.Volumes {
		switch volume.Type {
		case model.VolumeTypePersistent:
			persistentVolume = volume
		case model.VolumeTypeShared:
			sharedVolume = volume
		}
	}

	var persistentClaim, sharedClaim helm.Node
	for _, claim := range claims {
		switch claim.Get("metadata", "name").String() {
		case string(persistentVolume.Tag):
			persistentClaim = claim
		case string(sharedVolume.Tag):
			sharedClaim = claim
		default:
			assert.Fail("Got unexpected claim", "%s", claim)
		}
	}

	config := map[string]interface{}{
		"Values.kube.storage_class.persistent":              "Persistent",
		"Values.kube.storage_class.shared":                  "Shared",
		"Values.sizing.myrole.disk_sizes.persistent_volume": "42",
		"Values.sizing.myrole.disk_sizes.shared_volume":     "84",
	}

	actual, err := RoundtripNode(persistentClaim, config)
	if assert.NoError(err) {
		testhelpers.IsYAMLEqualString(assert, `---
		metadata:
			name: "persistent-volume"
			annotations:
				volume.beta.kubernetes.io/storage-class: "Persistent"
		spec:
			accessModes:
			-	"ReadWriteOnce"
			resources:
				requests:
					storage: "42G"
		`, actual)
	}

	actual, err = RoundtripNode(sharedClaim, config)
	if assert.NoError(err) {
		testhelpers.IsYAMLEqualString(assert, `---
		metadata:
			name: "shared-volume"
			annotations:
				volume.beta.kubernetes.io/storage-class: "Shared"
		spec:
			accessModes:
			-	"ReadWriteMany"
			resources:
				requests:
					storage: "84G"
		`, actual)
	}
}

func TestPodGetVolumeMounts(t *testing.T) {
	t.Parallel()
	role := podTemplateTestLoadRole(assert.New(t))
	if role == nil {
		return
	}

	cases := map[string]bool{
		"with hostpath":    true,
		"without hostpath": false,
	}
	for caseName, hasHostpath := range cases {
		t.Run(caseName, func(t *testing.T) {

			config := map[string]interface{}{
				"Values.kube.hostpath_available": hasHostpath,
				"Values.bosh.foo":                "bar",
			}
			volumeMountNodes := getVolumeMounts(role, ExportSettings{CreateHelmChart: true})
			volumeMounts, err := RoundtripNode(volumeMountNodes, config)
			if !assert.NoError(t, err) {
				return
			}
			if hasHostpath {
				assert.Len(t, volumeMounts, 4)
			} else {
				assert.Len(t, volumeMounts, 3)
			}

			var persistentMount, sharedMount, hostMount, deploymentManifestMount map[interface{}]interface{}
			for _, elem := range volumeMounts.([]interface{}) {
				mount := elem.(map[interface{}]interface{})
				switch mount["name"] {
				case "persistent-volume":
					persistentMount = mount
				case "shared-volume":
					sharedMount = mount
				case "host-volume":
					hostMount = mount
					sharedMount = mount
				case "deployment-manifest":
					deploymentManifestMount = mount
				default:
					assert.Fail(t, "Got unexpected volume mount", "%+v", mount)
				}
			}
			assert.Equal(t, "/mnt/persistent", persistentMount["mountPath"])
			assert.Equal(t, false, persistentMount["readOnly"])
			assert.Equal(t, "/mnt/shared", sharedMount["mountPath"])
			assert.Equal(t, false, sharedMount["readOnly"])
			assert.Equal(t, false, persistentMount["readOnly"])
			assert.Equal(t, "/opt/fissile/config", deploymentManifestMount["mountPath"])
			assert.Equal(t, true, deploymentManifestMount["readOnly"])
			if hasHostpath {
				assert.Equal(t, "/sys/fs/cgroup", hostMount["mountPath"])
				assert.Equal(t, false, hostMount["readOnly"])
			} else {
				assert.Nil(t, hostMount)
			}
		})
	}
}

func TestPodGetEnvVarsFromConfigSizingCountKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SIZING_FOO_COUNT",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
					Run: &model.RoleRun{
						Scaling: &model.RoleRunScaling{
							Min: 33,
						},
					},
				},
			},
		},
	})

	actual, err := RoundtripNode(ev, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SIZING_FOO_COUNT"
			value: "33"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigSizingCountHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SIZING_FOO_COUNT",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
					Run: &model.RoleRun{
						Scaling: &model.RoleRunScaling{
							Min: 1,
							HA:  7,
						},
					},
				},
			},
		},
	})

	config := map[string]interface{}{
		"Values.sizing.foo.count": "22",
	}

	actual, err := RoundtripNode(ev, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SIZING_FOO_COUNT"
			value: "22"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)

	config = map[string]interface{}{
		"Values.sizing.foo.count": 1, // Run.Scaling.Min
		"Values.config.HA":        true,
	}

	actual, err = RoundtripNode(ev, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SIZING_FOO_COUNT"
			value: "7"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigSizingPortsKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MIN",
		},
		&model.VariableDefinition{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MAX",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
					JobReferences: []*model.JobReference{
						&model.JobReference{
							ContainerProperties: model.JobContainerProperties{
								BoshContainerization: model.JobBoshContainerization{
									Ports: []model.JobExposedPort{
										model.JobExposedPort{
											Name:                "store",
											CountIsConfigurable: true,
											InternalPort:        333,
											Count:               55,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	actual, err := RoundtripNode(ev, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SIZING_FOO_PORTS_STORE_MAX"
			value: "387"
		-	name: "KUBE_SIZING_FOO_PORTS_STORE_MIN"
			value: "333"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigSizingPortsHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MIN",
		},
		&model.VariableDefinition{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MAX",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
					JobReferences: []*model.JobReference{
						&model.JobReference{
							ContainerProperties: model.JobContainerProperties{
								BoshContainerization: model.JobBoshContainerization{
									Ports: []model.JobExposedPort{
										model.JobExposedPort{
											Name:                "store",
											CountIsConfigurable: true,
											InternalPort:        333,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	config := map[string]interface{}{
		"Values.sizing.foo.ports.store.count": "22",
	}

	actual, err := RoundtripNode(ev, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SIZING_FOO_PORTS_STORE_MAX"
			value: "354"
		-	name: "KUBE_SIZING_FOO_PORTS_STORE_MIN"
			value: "333"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationCounterKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SECRETS_GENERATION_COUNTER",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})

	actual, err := RoundtripNode(ev, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SECRETS_GENERATION_COUNTER"
			value: "1"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationCounterHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SECRETS_GENERATION_COUNTER",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})

	config := map[string]interface{}{
		"Values.kube.secrets_generation_counter": "3",
	}

	actual, err := RoundtripNode(ev, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SECRETS_GENERATION_COUNTER"
			value: "3"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationNameKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SECRETS_GENERATION_NAME",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})

	actual, err := RoundtripNode(ev, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SECRETS_GENERATION_NAME"
			value: "secrets-1"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationNameHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "KUBE_SECRETS_GENERATION_NAME",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})

	config := map[string]interface{}{
		"Chart.Version":                          "CV",
		"Values.kube.secrets_generation_counter": "SGC",
	}

	actual, err := RoundtripNode(ev, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "KUBE_SECRETS_GENERATION_NAME"
			value: "secrets-CV-SGC"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigSecretsKube(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "A_SECRET",
			CVOptions: model.CVOptions{
				Secret: true,
			},
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})

	actual, err := RoundtripNode(ev, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "A_SECRET"
			valueFrom:
				secretKeyRef:
					key: "a-secret"
					name: "secrets"
		-	name: "KUBERNETES_NAMESPACE"
			valueFrom:
				fieldRef:
					fieldPath: "metadata.namespace"
		-	name: "VCAP_HARD_NPROC"
			value: "2048"
		-	name: "VCAP_SOFT_NPROC"
			value: "1024"
	`, actual)
}

func TestPodGetEnvVarsFromConfigSecretsHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	settings := ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	}

	t.Run("Plain", func(t *testing.T) {
		t.Parallel()
		ev, err := getEnvVarsFromConfigs(model.Variables{
			&model.VariableDefinition{
				Name: "A_SECRET",
				CVOptions: model.CVOptions{
					Secret: true,
				},
			},
		}, settings)
		if !assert.NoError(err) {
			return
		}

		actual, err := RoundtripNode(ev, nil)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "A_SECRET"
				valueFrom:
					secretKeyRef:
						key: "a-secret"
						name: "secrets"
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})

	t.Run("Generated", func(t *testing.T) {
		t.Parallel()
		cv := model.Variables{
			&model.VariableDefinition{
				Name: "A_SECRET",
				Type: "password",
				CVOptions: model.CVOptions{
					Secret: true,
				},
			},
		}

		config := map[string]interface{}{
			"Chart.Version":                          "CV",
			"Values.kube.secrets_generation_counter": "SGC",
			"Values.secrets.A_SECRET":                "",
		}

		ev, err := getEnvVarsFromConfigs(cv, settings)
		if !assert.NoError(err) {
			return
		}

		// Mutation of cv below between tests prevents parallel execution

		t.Run("AsIs", func(t *testing.T) {
			actual, err := RoundtripNode(ev, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				-	name: "A_SECRET"
					valueFrom:
						secretKeyRef:
							key: "a-secret"
							name: "secrets-CV-SGC"

				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
			`, actual)
		})

		t.Run("Overidden", func(t *testing.T) {
			config := map[string]interface{}{
				"Values.secrets.A_SECRET": "user's choice",
			}

			actual, err := RoundtripNode(ev, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				-	name: "A_SECRET"
					valueFrom:
						secretKeyRef:
							key: "a-secret"
							name: "secrets"

				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
			`, actual)
		})

		cv[0].CVOptions.Immutable = true
		ev, err = getEnvVarsFromConfigs(cv, settings)
		if !assert.NoError(err) {
			return
		}

		t.Run("Immutable", func(t *testing.T) {
			actual, err := RoundtripNode(ev, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				-	name: "A_SECRET"
					valueFrom:
						secretKeyRef:
							key: "a-secret"
							name: "secrets-CV-SGC"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
			`, actual)
		})
	})
}

func TestPodGetEnvVarsFromConfigNonSecretKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	settings := ExportSettings{
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	}

	t.Run("Present", func(t *testing.T) {
		t.Parallel()
		ev, err := getEnvVarsFromConfigs(model.Variables{
			&model.VariableDefinition{
				Name: "SOMETHING",
				CVOptions: model.CVOptions{
					Default: []string{"or", "other"},
				},
			},
		}, settings)

		actual, err := RoundtripNode(ev, nil)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: "[\"or\",\"other\"]"
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})

	t.Run("Missing", func(t *testing.T) {
		t.Parallel()
		ev, err := getEnvVarsFromConfigs(model.Variables{
			&model.VariableDefinition{
				Name: "SOMETHING",
			},
			&model.VariableDefinition{
				Name: "HOSTNAME",
				CVOptions: model.CVOptions{
					Type: model.CVTypeEnv,
				},
			},
		}, settings)

		actual, err := RoundtripNode(ev, nil)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: ""
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})
}

func TestPodGetEnvVarsFromConfigNonSecretHelmUserOptional(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "SOMETHING",
			CVOptions: model.CVOptions{
				Type: model.CVTypeUser,
			},
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})
	if !assert.NoError(err) {
		return
	}

	t.Run("Missing", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.env.SOMETHING": nil,
		}
		actual, err := RoundtripNode(ev, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: ""
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})

	t.Run("Present", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.env.SOMETHING": "else",
		}

		actual, err := RoundtripNode(ev, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: "else"
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})
}

func TestPodGetEnvVarsFromConfigNonSecretHelmUserRequired(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "SOMETHING",
			CVOptions: model.CVOptions{
				Type:     model.CVTypeUser,
				Required: true,
			},
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})
	require.NoError(t, err)

	t.Run("Missing", func(t *testing.T) {
		t.Parallel()
		_, err := RenderNode(ev, nil)
		assert.EqualError(err,
			`template: :7:219: executing "" at <fail "env.SOMETHING has not been set">: error calling fail: env.SOMETHING has not been set`)
	})

	t.Run("Undefined", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.env.SOMETHING": nil,
		}
		_, err := RenderNode(ev, config)
		assert.EqualError(err,
			`template: :7:219: executing "" at <fail "env.SOMETHING has not been set">: error calling fail: env.SOMETHING has not been set`)
	})

	t.Run("Present", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.env.SOMETHING": "needed",
		}

		actual, err := RoundtripNode(ev, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: "needed"
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})

	t.Run("Structured", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.env.SOMETHING": map[string]string{"foo": "bar"},
		}

		actual, err := RoundtripNode(ev, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: "{\"foo\":\"bar\"}"
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})
}

func TestPodGetEnvVarsFromConfigNonSecretHelmImagename(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs(model.Variables{
		&model.VariableDefinition{
			Name: "IMAGENAME",
			CVOptions: model.CVOptions{
				Type:      model.CVTypeUser,
				ImageName: true,
			},
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			InstanceGroups: []*model.InstanceGroup{
				&model.InstanceGroup{
					Name: "foo",
				},
			},
		},
	})
	if !assert.NoError(err) {
		return
	}

	t.Run("WithoutOrg", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.organization": "my-org",
			"Values.env.IMAGENAME":     "my-image:my-tag",
		}
		actual, err := RoundtripNode(ev, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "IMAGENAME"
				value: "docker.io/my-org/my-image:my-tag"
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})

	t.Run("WithOrg", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.organization": "my-org",
			"Values.env.IMAGENAME":     "org/image:tag",
		}

		actual, err := RoundtripNode(ev, config)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "IMAGENAME"
				value: "org/image:tag"
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "VCAP_HARD_NPROC"
				value: "2048"
			-	name: "VCAP_SOFT_NPROC"
				value: "1024"
		`, actual)
	})
}

func TestPodGetContainerLivenessProbe(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	samples := []Sample{
		{
			desc:     "Always on, defaults",
			input:    nil,
			expected: `---`,
		},
		{
			desc: "Port probe",
			input: &model.HealthProbe{
				Port: 1234,
			},
			expected: `---
				initialDelaySeconds: 600
				tcpSocket:
					port: 1234`,
		},
		{
			desc: "Command probe",
			input: &model.HealthProbe{
				Command: []string{"rm", "-rf", "--no-preserve-root", "/"},
			},
			expected: `---
				initialDelaySeconds: 600
				exec:
					command: [ rm, "-rf", "--no-preserve-root", /]`,
		},
		{
			desc: "URL probe (simple)",
			input: &model.HealthProbe{
				URL: "http://example.com/path",
			},
			expected: `---
				initialDelaySeconds: 600
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"`,
		},
		{
			desc: "URL probe (custom port)",
			input: &model.HealthProbe{
				URL: "https://example.com:1234/path",
			},
			expected: `---
				initialDelaySeconds: 600
				httpGet:
					scheme: HTTPS
					host:   "example.com"
					port:   1234
					path:   "/path"`,
		},
		{
			desc: "URL probe (Invalid scheme)",
			input: &model.HealthProbe{
				URL: "file:///etc/shadow",
			},
			err: "Health check for myrole has unsupported URI scheme \"file\"",
		},
		{
			desc: "URL probe (query)",
			input: &model.HealthProbe{
				URL: "http://example.com/path?query#hash",
			},
			expected: `---
				initialDelaySeconds: 600
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path?query"`,
		},
		{
			desc: "URL probe (auth)",
			input: &model.HealthProbe{
				URL: "http://user:pass@example.com/path",
			},
			// base64.StdEncoding.EncodeToString([]byte("user:pass")) is "dXNlcjpwYXNz"
			expected: `---
				initialDelaySeconds: 600
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"
					httpHeaders:
					-	name:  Authorization
						value: dXNlcjpwYXNz`,
		},
		{
			desc: "URL probe (custom headers)",
			input: &model.HealthProbe{
				URL:     "http://example.com/path",
				Headers: map[string]string{"x-header": "some value"},
			},
			expected: `---
				initialDelaySeconds: 600
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"
					httpHeaders:
					-	name:  "X-Header"
						value: "some value"`,
		},
		{
			desc: "URL probe (invalid URL)",
			input: &model.HealthProbe{
				URL: "://",
			},
			err: "Invalid liveness URL health check for myrole: parse ://: missing protocol scheme",
		},
		{
			desc: "URL probe (invalid port)",
			input: &model.HealthProbe{
				URL: "http://example.com:port_number/",
			},
			err: "Failed to get URL port for health check for myrole: invalid host \"example.com:port_number\"",
		},
		{
			desc: "URL probe (localhost)",
			input: &model.HealthProbe{
				URL: "http://container-ip/path",
			},
			expected: `---
				initialDelaySeconds: 600
				httpGet:
					scheme: HTTP
					port:   80
					path:   "/path"`,
		},
		{
			desc: "Port probe, liveness timeout",
			input: &model.HealthProbe{
				Port:    1234,
				Timeout: 20,
			},
			expected: `---
				timeoutSeconds:      20
				initialDelaySeconds: 600
				tcpSocket:
					port: 1234`,
		},
		{
			desc: "Command probe, liveness timeout",
			input: &model.HealthProbe{
				Command: []string{"rm", "-rf", "--no-preserve-root", "/"},
				Timeout: 20,
			},
			expected: `---
				timeoutSeconds:      20
				initialDelaySeconds: 600
				exec:
					command: [ rm, "-rf", "--no-preserve-root", /]`,
		},
		{
			desc: "URL probe (simple), liveness timeout",
			input: &model.HealthProbe{
				URL:     "http://example.com/path",
				Timeout: 20,
			},
			expected: `---
				timeoutSeconds:      20
				initialDelaySeconds: 600
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"`,
		},
		{
			desc: "Initial Delay Seconds",
			input: &model.HealthProbe{
				InitialDelay: 22,
				Port:         2289,
			},
			expected: `---
				initialDelaySeconds: 22
				tcpSocket:
					port: 2289`,
		},
		{
			desc: "Success Threshold - Properly IGNORED",
			input: &model.HealthProbe{
				SuccessThreshold: 20,
				Port:             2289,
			},
			expected: `---
				initialDelaySeconds: 600
				tcpSocket:
					port: 2289`,
		},
		{
			desc: "Failure Threshold",
			input: &model.HealthProbe{
				FailureThreshold: 20,
				Port:             2289,
			},
			expected: `---
				failureThreshold:    20
				initialDelaySeconds: 600
				tcpSocket:
					port: 2289`,
		},
		{
			desc: "Period Seconds",
			input: &model.HealthProbe{
				Period: 20,
				Port:   2289,
			},
			expected: `---
				periodSeconds:       20
				initialDelaySeconds: 600
				tcpSocket:
					port: 2289`,
		},
	}

	for _, sample := range samples {
		probe, _ := sample.input.(*model.HealthProbe)
		role.Run.HealthCheck = &model.HealthCheck{Liveness: probe}
		actual, err := getContainerLivenessProbe(role)
		sample.check(t, actual, err)
	}
}

func TestPodGetContainerReadinessProbe(t *testing.T) {
	t.Parallel()

	type sampleStruct struct {
		desc  string
		input *model.HealthProbe
		// We have different expected behaviours for docker roles (supporting
		// all three probe types) and BOSH roles (only commands are allowed)
		dockerExpected string
		dockerError    string
		boshExpected   string
		boshError      string
	}

	samples := []sampleStruct{
		{
			desc:           "No probe",
			input:          nil,
			dockerExpected: ``,
			boshExpected: `---
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "Port probe",
			input: &model.HealthProbe{
				Port: 1234,
			},
			dockerExpected: `---
				tcpSocket:
					port: 1234`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "Command probe",
			input: &model.HealthProbe{
				Command: []string{"rm", "-rf", "--no-preserve-root", "/"},
			},
			dockerExpected: `---
				exec:
					command: [ rm, "-rf", "--no-preserve-root", /]`,
			boshExpected: `---
				exec:
					# Note that this is being interpreted as five separate commands
					command: [ /opt/fissile/readiness-probe.sh, rm, "-rf", "--no-preserve-root", /]`,
		},
		{
			desc: "URL probe (simple)",
			input: &model.HealthProbe{
				URL: "http://example.com/path",
			},
			dockerExpected: `---
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (custom port)",
			input: &model.HealthProbe{
				URL: "https://example.com:1234/path",
			},
			dockerExpected: `---
				httpGet:
					scheme: HTTPS
					host:   "example.com"
					port:   1234
					path:   "/path"`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (Invalid scheme)",
			input: &model.HealthProbe{
				URL: "file:///etc/shadow",
			},
			dockerError: "Health check for myrole has unsupported URI scheme \"file\"",
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (query)",
			input: &model.HealthProbe{
				URL: "http://example.com/path?query#hash",
			},
			dockerExpected: `---
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path?query"`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (auth)",
			input: &model.HealthProbe{
				URL: "http://user:pass@example.com/path",
			},
			dockerExpected: `---
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"
					httpHeaders:
					-	name:  Authorization
						value: dXNlcjpwYXNz`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (custom headers)",
			input: &model.HealthProbe{
				URL:     "http://example.com/path",
				Headers: map[string]string{"x-header": "some value"},
			},
			dockerExpected: `---
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"
					httpHeaders:
					-	name:  "X-Header"
						value: "some value"`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (invalid URL)",
			input: &model.HealthProbe{
				URL: "://",
			},
			dockerError: "Invalid readiness URL health check for myrole: parse ://: missing protocol scheme",
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (invalid port)",
			input: &model.HealthProbe{
				URL: "http://example.com:port_number/",
			},
			dockerError: "Failed to get URL port for health check for myrole: invalid host \"example.com:port_number\"",
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "URL probe (localhost)",
			input: &model.HealthProbe{
				URL: "http://container-ip/path",
			},
			dockerExpected: `---
				httpGet:
					scheme: HTTP
					port:   80
					path:   "/path"`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "Port probe, readiness timeout",
			input: &model.HealthProbe{
				Port:    1234,
				Timeout: 20,
			},
			dockerExpected: `---
				timeoutSeconds: 20
				tcpSocket:
					port: 1234`,
			boshExpected: `---
				# This would have failed validation
				timeoutSeconds: 20
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "Command probe, readiness timeout",
			input: &model.HealthProbe{
				Command: []string{"rm", "-rf", "--no-preserve-root", "/"},
				Timeout: 20,
			},
			dockerExpected: `---
				timeoutSeconds: 20
				exec:
					command: [ rm, "-rf", "--no-preserve-root", /]`,
			boshExpected: `---
				timeoutSeconds: 20
				exec:
					# This is interpreted as five separate commands
					command: [ /opt/fissile/readiness-probe.sh, rm, "-rf", "--no-preserve-root", /]`,
		},
		{
			desc: "URL probe (simple), readiness timeout",
			input: &model.HealthProbe{
				URL:     "http://example.com/path",
				Timeout: 20,
			},
			dockerExpected: `---
				timeoutSeconds: 20
				httpGet:
					scheme: HTTP
					host:   "example.com"
					port:   80
					path:   "/path"`,
			boshExpected: `---
				# This would have failed validation
				exec:
					command: [ /opt/fissile/readiness-probe.sh ]`,
		},
		{
			desc: "Initial Delay Seconds",
			input: &model.HealthProbe{
				InitialDelay: 22,
				Command:      []string{"/bin/true"},
			},
			dockerExpected: `---
				initialDelaySeconds: 22
				exec:
					command: [ /bin/true ]`,
			boshExpected: `---
				initialDelaySeconds: 22
				exec:
					command: [ /opt/fissile/readiness-probe.sh, /bin/true ]`,
		},
		{
			desc: "Success Threshold",
			input: &model.HealthProbe{
				SuccessThreshold: 20,
				Command:          []string{"/bin/true"},
			},
			dockerExpected: `---
				successThreshold: 20
				exec:
					command: [ /bin/true ]`,
			boshExpected: `---
				successThreshold: 20
				exec:
					command: [ /opt/fissile/readiness-probe.sh, /bin/true ]`,
		},
		{
			desc: "Failure Threshold",
			input: &model.HealthProbe{
				FailureThreshold: 20,
				Command:          []string{"/bin/true"},
			},
			dockerExpected: `---
				failureThreshold: 20
				exec:
					command: [ /bin/true ]`,
			boshExpected: `---
				failureThreshold: 20
				exec:
					command: [ /opt/fissile/readiness-probe.sh, /bin/true ]`,
		},
		{
			desc: "Period Seconds",
			input: &model.HealthProbe{
				Period:  20,
				Command: []string{"/bin/true"},
			},
			dockerExpected: `---
				periodSeconds: 20
				exec:
					command: [ /bin/true ]`,
			boshExpected: `---
				periodSeconds: 20
				exec:
					command: [ /opt/fissile/readiness-probe.sh, /bin/true ]`,
		},
	}

	for _, sample := range samples {
		func(sample sampleStruct) {
			t.Run(sample.desc, func(t *testing.T) {
				t.Parallel()
				t.Run("bosh", func(t *testing.T) {
					t.Parallel()
					role := podTemplateTestLoadRole(assert.New(t))
					require.NotNil(t, role)
					role.Run.HealthCheck = &model.HealthCheck{Readiness: sample.input}
					role.Type = model.RoleTypeBosh
					probe, err := getContainerReadinessProbe(role)
					if sample.boshError != "" {
						assert.EqualError(t, err, sample.boshError)
						return
					}
					require.NoError(t, err)
					if sample.boshExpected == "" {
						assert.Nil(t, probe)
						return
					}
					require.NotNil(t, probe, "No error getting readiness probe but it was nil")
					t.Run("kube", func(t *testing.T) {
						t.Parallel()
						actual, err := RoundtripKube(probe)
						if assert.NoError(t, err) {
							// We use subset testing here because we don't want to bother with the
							// default timeout lengths
							testhelpers.IsYAMLSubsetString(assert.New(t), sample.boshExpected, actual)
						}
					})
					t.Run("helm", func(t *testing.T) {
						t.Parallel()
						actual, err := RoundtripNode(probe, map[string]interface{}{})
						if assert.NoError(t, err) {
							// We use subset testing here because we don't want to bother with the
							// default timeout lengths
							testhelpers.IsYAMLSubsetString(assert.New(t), sample.boshExpected, actual)
						}
					})
				})
			})
		}(sample)
	}
}

func podTestLoadRoleFrom(assert *assert.Assertions, roleName, manifestName string) *model.InstanceGroup {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/kube", manifestName)
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	manifest, err := loader.LoadRoleManifest(manifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{releasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	if !assert.NoError(err) {
		return nil
	}
	role := manifest.LookupInstanceGroup(roleName)
	if !assert.NotNil(role, "Failed to find role %s", roleName) {
		return nil
	}

	return role
}

func podTestLoadRole(assert *assert.Assertions, roleName string) *model.InstanceGroup {
	return podTestLoadRoleFrom(assert, roleName, "pods.yml")
}

func TestPodPreFlightKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	actual, err := RoundtripNode(pod, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLSubsetString(assert, `---
		apiVersion: v1
		kind: Pod
		metadata:
			name: pre-role
		spec:
			containers:
			-
				name: pre-role
			restartPolicy: OnFailure
			terminationGracePeriodSeconds: 600
	`, actual)
}

func TestPodPreFlightHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	config := map[string]interface{}{
		"Values.kube.registry.hostname":        "R",
		"Values.kube.registry.username":        "U",
		"Values.kube.organization":             "O",
		"Values.env.KUBERNETES_CLUSTER_DOMAIN": "cluster.local",
		"Values.sizing.pre_role.capabilities":  []interface{}{},
	}

	actual, err := RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				app.kubernetes.io/component: pre-role
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_CLUSTER_DOMAIN"
					value: "cluster.local"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
				image: "R/O/theRepo-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
				lifecycle:
					preStop:
						exec:
							command:
							-	"/opt/fissile/pre-stop.sh"
				livenessProbe: ~
				name: "pre-role"
				ports: ~
				readinessProbe: ~
				resources: ~
				securityContext:
					allowPrivilegeEscalation: false
					capabilities:
						add:	~
				volumeMounts:
				-	mountPath: /opt/fissile/config
					name: deployment-manifest
					readOnly: true
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes:
			-	name: deployment-manifest
				secret:
					items:
					-	key: deployment-manifest
						path: deployment-manifest.yml
					secretName: deployment-manifest
	`, actual)
}

func TestPodPostFlightKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "post-role")
	if role == nil {
		return
	}

	pod, err := NewPod(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role post-role") {
		return
	}
	assert.NotNil(pod)

	actual, err := RoundtripNode(pod, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLSubsetString(assert, `---
		apiVersion: v1
		kind: Pod
		metadata:
			name: post-role
		spec:
			containers:
			-
				name: post-role
				resources: ~
			restartPolicy: OnFailure
	`, actual)
}

func TestPodPostFlightHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "post-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role post-role") {
		return
	}
	assert.NotNil(pod)

	config := map[string]interface{}{
		"Values.kube.registry.hostname":        "R",
		"Values.kube.registry.username":        "U",
		"Values.kube.organization":             "O",
		"Values.env.KUBERNETES_CLUSTER_DOMAIN": "cluster.local",
		"Values.sizing.post_role.capabilities": []interface{}{},
	}

	actual, err := RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "post-role"
			labels:
				app.kubernetes.io/component: post-role
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "post-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_CLUSTER_DOMAIN"
					value: "cluster.local"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
				image: "R/O/theRepo-post-role:e9f459d3c3576bf1129a6b18ca2763f73fa19645"
				lifecycle:
					preStop:
						exec:
							command:
							-	"/opt/fissile/pre-stop.sh"
				livenessProbe: ~
				name: "post-role"
				ports: ~
				readinessProbe: ~
				resources: ~
				securityContext:
					allowPrivilegeEscalation: false
					capabilities:
						add:	~
				volumeMounts:
				-	mountPath: /opt/fissile/config
					name: deployment-manifest
					readOnly: true
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes:
			-	name: deployment-manifest
				secret:
					items:
					-	key: deployment-manifest
						path: deployment-manifest.yml
					secretName: deployment-manifest
	`, actual)
}

func TestPodMemoryKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}

	pod, err := NewPod(role, ExportSettings{
		Opinions:        model.NewEmptyOpinions(),
		UseMemoryLimits: true,
	}, nil)

	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	actual, err := RoundtripNode(pod, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLSubsetString(assert, `---
		apiVersion: v1
		kind: Pod
		metadata:
			name: pre-role
		spec:
			containers:
			-
				name: pre-role
				resources:
					requests:
						memory: 128Mi
					limits:
						memory: 384Mi
			restartPolicy: OnFailure
			terminationGracePeriodSeconds: 600
	`, actual)
}

func TestPodMemoryHelmDisabled(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
		UseMemoryLimits: true,
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	config := map[string]interface{}{
		"Values.config.memory.requests":         nil,
		"Values.kube.registry.hostname":         "R",
		"Values.kube.registry.username":         "U",
		"Values.kube.organization":              "O",
		"Values.env.KUBERNETES_CLUSTER_DOMAIN":  "cluster.local",
		"Values.sizing.pre_role.capabilities":   []interface{}{},
		"Values.sizing.pre_role.memory.request": nil,
	}

	actual, err := RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				app.kubernetes.io/component: pre-role
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_CLUSTER_DOMAIN"
					value: "cluster.local"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
				image: "R/O/theRepo-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
				lifecycle:
					preStop:
						exec:
							command:
							-	"/opt/fissile/pre-stop.sh"
				livenessProbe: ~
				name: "pre-role"
				ports: ~
				readinessProbe: ~
				resources:
					requests:
					limits:
				securityContext:
					allowPrivilegeEscalation: false
					capabilities:
						add:	~
				volumeMounts:
				-	mountPath: /opt/fissile/config
					name: deployment-manifest
					readOnly: true
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes:
			-	name: deployment-manifest
				secret:
					items:
					-	key: deployment-manifest
						path: deployment-manifest.yml
					secretName: deployment-manifest
	`, actual)
}

func TestPodMemoryHelmActive(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
		UseMemoryLimits: true,
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	config := map[string]interface{}{
		"Values.config.memory.limits":           "true",
		"Values.config.memory.requests":         "true",
		"Values.env.KUBERNETES_CLUSTER_DOMAIN":  "cluster.local",
		"Values.kube.organization":              "O",
		"Values.kube.registry.hostname":         "R",
		"Values.kube.registry.username":         "U",
		"Values.sizing.pre_role.capabilities":   []interface{}{},
		"Values.sizing.pre_role.memory.limit":   "10",
		"Values.sizing.pre_role.memory.request": "1",
	}

	actual, err := RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				app.kubernetes.io/component: pre-role
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_CLUSTER_DOMAIN"
					value: "cluster.local"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
				image: "R/O/theRepo-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
				lifecycle:
					preStop:
						exec:
							command:
							-	"/opt/fissile/pre-stop.sh"
				livenessProbe: ~
				name: "pre-role"
				ports: ~
				readinessProbe: ~
				resources:
					requests:
						memory: "1Mi"
					limits:
						memory: "10Mi"
				securityContext:
					allowPrivilegeEscalation: false
					capabilities:
						add:	~
				volumeMounts:
				-	mountPath: /opt/fissile/config
					name: deployment-manifest
					readOnly: true
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes:
			-	name: deployment-manifest
				secret:
					items:
					-	key: deployment-manifest
						path: deployment-manifest.yml
					secretName: deployment-manifest
	`, actual)
}

func TestPodCPUKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		Opinions:     model.NewEmptyOpinions(),
		UseCPULimits: true,
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	actual, err := RoundtripKube(pod)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLSubsetString(assert, `---
		apiVersion: v1
		kind: Pod
		metadata:
			name: pre-role
		spec:
			containers:
			-
				name: pre-role
				resources:
					requests:
						cpu: 2000m
					limits:
						cpu: 4000m
			restartPolicy: OnFailure
			terminationGracePeriodSeconds: 600
	`, actual)
}

func TestPodCPUHelmDisabled(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
		UseCPULimits:    true,
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	config := map[string]interface{}{
		"Values.config.cpu.requests":           nil,
		"Values.env.KUBERNETES_CLUSTER_DOMAIN": "cluster.local",
		"Values.kube.organization":             "O",
		"Values.kube.registry.hostname":        "R",
		"Values.kube.registry.username":        "U",
		"Values.sizing.pre_role.capabilities":  []interface{}{},
		"Values.sizing.pre_role.cpu.request":   nil,
	}

	actual, err := RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				app.kubernetes.io/component: pre-role
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_CLUSTER_DOMAIN"
					value: "cluster.local"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
				image: "R/O/theRepo-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
				lifecycle:
					preStop:
						exec:
							command:
							-	"/opt/fissile/pre-stop.sh"
				livenessProbe: ~
				name: "pre-role"
				ports: ~
				readinessProbe: ~
				resources:
					requests:
					limits:
				securityContext:
					allowPrivilegeEscalation: false
					capabilities:
						add:	~
				volumeMounts:
				-	mountPath: /opt/fissile/config
					name: deployment-manifest
					readOnly: true
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes:
			-	name: deployment-manifest
				secret:
					items:
					-	key: deployment-manifest
						path: deployment-manifest.yml
					secretName: deployment-manifest
	`, actual)
}

func TestPodCPUHelmActive(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "pre-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
		UseCPULimits:    true,
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role pre-role") {
		return
	}
	assert.NotNil(pod)

	config := map[string]interface{}{
		"Values.config.cpu.limits":             "true",
		"Values.config.cpu.requests":           "true",
		"Values.env.KUBERNETES_CLUSTER_DOMAIN": "cluster.local",
		"Values.kube.organization":             "O",
		"Values.kube.registry.hostname":        "R",
		"Values.kube.registry.username":        "U",
		"Values.sizing.pre_role.capabilities":  []interface{}{},
		"Values.sizing.pre_role.cpu.limit":     "10",
		"Values.sizing.pre_role.cpu.request":   "1",
	}

	actual, err := RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				app.kubernetes.io/component: pre-role
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_CLUSTER_DOMAIN"
					value: "cluster.local"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
				image: "R/O/theRepo-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
				lifecycle:
					preStop:
						exec:
							command:
							-	"/opt/fissile/pre-stop.sh"
				livenessProbe: ~
				name: "pre-role"
				ports: ~
				readinessProbe: ~
				resources:
					requests:
						cpu: "1m"
					limits:
						cpu: "10m"
				securityContext:
					allowPrivilegeEscalation: false
					capabilities:
						add:	~
				volumeMounts:
				-	mountPath: /opt/fissile/config
					name: deployment-manifest
					readOnly: true
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes:
			-	name: deployment-manifest
				secret:
					items:
					-	key: deployment-manifest
						path: deployment-manifest.yml
					secretName: deployment-manifest
	`, actual)
}

func TestGetSecurityContextCapList(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	t.Run("Kube", func(t *testing.T) {
		t.Parallel()
		sc := getSecurityContext(role, false)
		if !assert.NotNil(sc) {
			return
		}

		actual, err := RoundtripKube(sc)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			allowPrivilegeEscalation: false
			capabilities:
				add:
				-	"SOMETHING"
		`, actual)
	})

	t.Run("Helm", func(t *testing.T) {
		t.Parallel()
		sc := getSecurityContext(role, true)
		if !assert.NotNil(sc) {
			return
		}

		t.Run("OverrideNone", func(t *testing.T) {
			t.Parallel()
			config := map[string]interface{}{
				"Values.sizing.myrole.capabilities": []interface{}{},
			}
			actual, err := RoundtripNode(sc, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				allowPrivilegeEscalation: false
				capabilities:
					add:
					-	"SOMETHING"
			`, actual)
		})

		t.Run("OverrideALL", func(t *testing.T) {
			t.Parallel()
			config := map[string]interface{}{
				"Values.sizing.myrole.capabilities": []interface{}{"ALL"},
			}
			actual, err := RoundtripNode(sc, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				# privileged: true implies allowPrivilegeEscalation
				allowPrivilegeEscalation: true
				privileged: true
			`, actual)
		})

		t.Run("OverrideSomething", func(t *testing.T) {
			t.Parallel()
			config := map[string]interface{}{
				"Values.sizing.myrole.capabilities": []interface{}{"something"},
			}
			actual, err := RoundtripNode(sc, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				allowPrivilegeEscalation: false
				capabilities:
					add:
					-	"SOMETHING"
					-	"SOMETHING"
			`, actual)
		})
	})
}

func TestGetSecurityContextNil(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	// Clear the capability list from the manifest to force default privileges.

	role.Run.Capabilities = []string{}

	t.Run("Kube", func(t *testing.T) {
		t.Parallel()
		sc := getSecurityContext(role, false)
		if !assert.NotNil(sc) {
			return
		}

		actual, err := RoundtripKube(sc)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			allowPrivilegeEscalation: false
		`, actual)
	})

	t.Run("Helm", func(t *testing.T) {
		t.Parallel()
		sc := getSecurityContext(role, true)
		if !assert.NotNil(sc) {
			return
		}

		t.Run("OverrideNone", func(t *testing.T) {
			t.Parallel()
			config := map[string]interface{}{
				"Values.sizing.myrole.capabilities": []interface{}{},
			}
			actual, err := RoundtripNode(sc, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				allowPrivilegeEscalation: false
				capabilities:
					add:	~
			`, actual)
		})

		t.Run("OverrideALL", func(t *testing.T) {
			t.Parallel()
			config := map[string]interface{}{
				"Values.sizing.myrole.capabilities": []interface{}{"ALL"},
			}
			actual, err := RoundtripNode(sc, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				# privileged: true implies allowPrivilegeEscalation
				allowPrivilegeEscalation: true
				privileged: true
			`, actual)
		})

		t.Run("OverrideSomething", func(t *testing.T) {
			t.Parallel()
			config := map[string]interface{}{
				"Values.sizing.myrole.capabilities": []interface{}{"something"},
			}
			actual, err := RoundtripNode(sc, config)
			if !assert.NoError(err) {
				return
			}
			testhelpers.IsYAMLEqualString(assert, `---
				allowPrivilegeEscalation: false
				capabilities:
					add:
					-	SOMETHING
			`, actual)
		})
	})
}

func TestGetSecurityContextPrivileged(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	// Override the capability from the manifest to force
	// privileged mode. As we are doing this after
	// LoadRoleManifest has normalized capabilities we have to use
	// all-uppercase now ourselves, to match the expectations of
	// the backend code.
	role.Run.Capabilities[0] = "ALL"

	t.Run("Kube", func(t *testing.T) {
		t.Parallel()
		sc := getSecurityContext(role, false)
		if !assert.NotNil(sc) {
			return
		}

		actual, err := RoundtripKube(sc)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			privileged: true
		`, actual)
	})

	t.Run("Helm", func(t *testing.T) {
		t.Parallel()
		sc := getSecurityContext(role, true)
		if !assert.NotNil(sc) {
			return
		}

		actual, err := RoundtripKube(sc)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			privileged: true
		`, actual)
	})
}

func TestPodGetContainerImageNameKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	settings := ExportSettings{
		Repository:   "theRepo",
		Opinions:     model.NewEmptyOpinions(),
		Organization: "O",
		Registry:     "R",
	}
	grapher := FakeGrapher{}

	name, err := getContainerImageName(role, settings, grapher)

	assert.Nil(err)
	assert.NotNil(name)
	assert.Equal(`R/O/theRepo-myrole:d0aca33ba5bc55dce697d9d57b46e1b23688659c`, name)
}

func TestPodGetContainerImageNameHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	settings := ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
	}
	grapher := FakeGrapher{}

	name, err := getContainerImageName(role, settings, grapher)

	assert.Nil(err)
	assert.NotNil(name)

	// Wrapping the name into a helm node for rendering
	// (avoid tests against the raw template)

	nameNode := helm.NewNode(name)

	config := map[string]interface{}{
		"Values.kube.registry.hostname": "R",
		"Values.kube.organization":      "O",
	}

	actual, err := RoundtripNode(nameNode, config)
	if !assert.NoError(err) {
		return
	}

	testhelpers.IsYAMLEqualString(assert, `---
		R/O/theRepo-myrole:d0aca33ba5bc55dce697d9d57b46e1b23688659c
	`, actual)
}

func TestPodGetContainerPortsKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRoleFrom(assert, "myrole", "exposed-ports.yml")
	if role == nil {
		return
	}

	settings := ExportSettings{}

	ports, err := getContainerPorts(role, settings)
	assert.Nil(err)
	assert.NotNil(ports)

	actual, err := RoundtripKube(ports)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	containerPort: 8080
			name: "http"
			protocol: "TCP"
		-	containerPort: 443
			name: "https"
			protocol: "TCP"
	`, actual)
}

func TestPodGetContainerPortsHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRoleFrom(assert, "myrole", "exposed-ports.yml")
	if role == nil {
		return
	}

	settings := ExportSettings{
		CreateHelmChart: true,
	}

	ports, err := getContainerPorts(role, settings)
	assert.Nil(err)
	assert.NotNil(ports)

	actual, err := RoundtripNode(ports, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	containerPort: 8080
			name: "http"
			protocol: "TCP"
		-	containerPort: 443
			name: "https"
			protocol: "TCP"
	`, actual)
}

func TestPodGetContainerPortsHelmCountConfigurable(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRoleFrom(assert, "myrole", "bosh-run-count-configurable.yml")
	if role == nil {
		return
	}

	settings := ExportSettings{
		CreateHelmChart: true,
	}

	ports, err := getContainerPorts(role, settings)
	assert.Nil(err)
	assert.NotNil(ports)

	config := map[string]interface{}{
		"Values.sizing.myrole.ports.tcp_route.count": "5",
	}

	actual, err := RoundtripNode(ports, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	containerPort: 20000
			name: "tcp-route-0"
			protocol: "TCP"
		-	containerPort: 20001
			name: "tcp-route-1"
			protocol: "TCP"
		-	containerPort: 20002
			name: "tcp-route-2"
			protocol: "TCP"
		-	containerPort: 20003
			name: "tcp-route-3"
			protocol: "TCP"
		-	containerPort: 20004
			name: "tcp-route-4"
			protocol: "TCP"
	`, actual)
}

func TestPodMakeSecretVarPlain(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	sv := makeSecretVar("foo", false)

	actual, err := RoundtripNode(sv, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		name: "foo"
		valueFrom:
			secretKeyRef:
				key: "foo"
				name: "secrets"
	`, actual)
}

func TestPodMakeSecretVarGenerated(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	sv := makeSecretVar("foo", true)

	config := map[string]interface{}{
		"Chart.Version":                          "CV",
		"Values.kube.secrets_generation_counter": "SGC",
	}

	actual, err := RoundtripNode(sv, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		name: "foo"
		valueFrom:
			secretKeyRef:
				key: "foo"
				name: "secrets-CV-SGC"
	`, actual)
}

func TestPodVolumeTypeEmptyDir(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/kube/colocated-containers.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(err)
	assert.NotNil(roleManifest)
	assert.NotNil(roleManifest.InstanceGroups)

	// Check non-claim volumes
	mounts := getNonClaimVolumes(roleManifest.LookupInstanceGroup("main-role"), ExportSettings{CreateHelmChart: true})
	assert.NotNil(mounts)
	actual, err := RoundtripNode(mounts, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "shared-data"
			emptyDir: {}
		-	name: deployment-manifest
			secret:
				items:
				-	key: deployment-manifest
					path: deployment-manifest.yml
				secretName: deployment-manifest
	`, actual)

	// Check each role for its volume mount
	for _, roleName := range []string{"main-role", "to-be-colocated"} {
		role := roleManifest.LookupInstanceGroup(roleName)

		mounts := getVolumeMounts(role, ExportSettings{CreateHelmChart: true})
		assert.NotNil(mounts)
		actual, err := RoundtripNode(mounts, nil)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			-	mountPath: "/var/vcap/store"
				name: "shared-data"
			-	mountPath: /opt/fissile/config
				name: deployment-manifest
				readOnly: true
		`, actual)
	}
}

func TestPodIstioManagedHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	role := podTestLoadRole(assert, "istio-managed-role")
	if role == nil {
		return
	}
	pod, err := NewPod(role, ExportSettings{
		CreateHelmChart: true,
		Repository:      "theRepo",
		Opinions:        model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create pod from role istio-managed-role") {
		return
	}
	assert.NotNil(pod)

	config := map[string]interface{}{
		"Values.config.use_istio":                       "true",
		"Values.kube.registry.hostname":                 "R",
		"Values.kube.registry.username":                 "U",
		"Values.kube.organization":                      "O",
		"Values.env.KUBERNETES_CLUSTER_DOMAIN":          "cluster.local",
		"Values.sizing.istio_managed_role.capabilities": []interface{}{},
	}

	actual, err := RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "istio-managed-role"
			labels:
				app: istio-managed-role
				app.kubernetes.io/component: istio-managed-role
				app.kubernetes.io/instance: MyRelease
				app.kubernetes.io/managed-by: Tiller
				app.kubernetes.io/name: MyChart
				app.kubernetes.io/version: 1.22.333.4444
				helm.sh/chart: MyChart-42.1_foo
				skiff-role-name: "istio-managed-role"
				version: 1.22.333.4444
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_CLUSTER_DOMAIN"
					value: "cluster.local"
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "VCAP_HARD_NPROC"
					value: "2048"
				-	name: "VCAP_SOFT_NPROC"
					value: "1024"
				image: "R/O/theRepo-istio-managed-role:e9f459d3c3576bf1129a6b18ca2763f73fa19645"
				lifecycle:
					preStop:
						exec:
							command:
							-	"/opt/fissile/pre-stop.sh"
				livenessProbe: ~
				name: "istio-managed-role"
				ports: ~
				readinessProbe:
					exec:
						command:
						- /opt/fissile/readiness-probe.sh
				resources: ~
				securityContext:
					allowPrivilegeEscalation: false
					capabilities:
						add:	~
				volumeMounts:
				-	mountPath: /opt/fissile/config
					name: deployment-manifest
					readOnly: true
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes:
			-	name: deployment-manifest
				secret:
					items:
					-	key: deployment-manifest
						path: deployment-manifest.yml
					secretName: deployment-manifest
	`, actual)
}
