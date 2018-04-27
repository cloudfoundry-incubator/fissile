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

func podTemplateTestLoadRole(assert *assert.Assertions) *model.Role {
	workDir, err := os.Getwd()
	if !assert.NoError(err) {
		return nil
	}

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/volumes.yml")
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")

	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return nil
	}
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release}, nil)
	if !assert.NoError(err) {
		return nil
	}
	role := manifest.LookupRole("myrole")
	if !assert.NotNil(role, "Failed to find role in manifest") {
		return nil
	}

	// Force a broadcast SECRET_VAR into the manifest, to see in all roles
	manifest.Configuration.Variables =
		append(manifest.Configuration.Variables,
			&model.ConfigurationVariable{
				Name:     "SECRET_VAR",
				Type:     model.CVTypeUser,
				Secret:   true,
				Internal: true,
			})
	return role
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
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	mounts := getNonClaimVolumes(role)
	assert.NotNil(mounts)

	actual, err := testhelpers.RoundtripNode(mounts, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		-	name: "host-volume"
			hostPath:
				path: "/sys/fs/cgroup"
				type: "Directory"
	`, actual)
}

func TestPodGetVolumes(t *testing.T) {
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

	actual, err := testhelpers.RoundtripNode(persistentClaim, config)
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

	actual, err = testhelpers.RoundtripNode(sharedClaim, config)
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
	role := podTemplateTestLoadRole(assert.New(t))
	if role == nil {
		return
	}

	cases := map[string]interface{}{
		"with hostpath":    nil,
		"without hostpath": map[string]interface{}{"Values.kube.hostpath_available": false},
	}
	for caseName, caseOverrides := range cases {
		t.Run(caseName, func(t *testing.T) {

			volumeMountNodes := getVolumeMounts(role)
			volumeMounts, err := testhelpers.RoundtripNode(volumeMountNodes, caseOverrides)
			if !assert.NoError(t, err) {
				return
			}
			if caseOverrides == nil {
				assert.Len(t, volumeMounts, 3)
			} else {
				assert.Len(t, volumeMounts, 2)
			}

			var persistentMount, sharedMount, hostMount map[interface{}]interface{}
			for _, elem := range volumeMounts.([]interface{}) {
				mount := elem.(map[interface{}]interface{})
				switch mount["name"] {
				case "persistent-volume":
					persistentMount = mount
				case "shared-volume":
					sharedMount = mount
				case "host-volume":
					hostMount = mount
				default:
					assert.Fail(t, "Got unexpected volume mount", "%+v", mount)
				}
			}
			assert.Equal(t, "/mnt/persistent", persistentMount["mountPath"])
			assert.Equal(t, false, persistentMount["readOnly"])
			assert.Equal(t, "/mnt/shared", sharedMount["mountPath"])
			assert.Equal(t, false, sharedMount["readOnly"])
			if caseOverrides == nil {
				assert.Equal(t, "/sys/fs/cgroup", hostMount["mountPath"])
				assert.Equal(t, false, hostMount["readOnly"])
			} else {
				assert.Nil(t, hostMount)
			}
		})
	}
}

func TestPodGetEnvVars(t *testing.T) {
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	if !assert.Equal(1, len(role.RoleJobs), "Role should have one job") {
		return
	}

	role.RoleJobs[0].Properties = []*model.JobProperty{
		&model.JobProperty{
			Name: "some-property",
		},
	}

	role.Configuration.Templates["properties.some-property"] = "((SOME_VAR))"

	samples := []Sample{
		{
			desc:  "Simple string",
			input: "simple string",
			expected: `---
				-	name: ALL_VAR
					value: placeholder
				-	name: KUBERNETES_NAMESPACE
					valueFrom:
						fieldRef:
							fieldPath: metadata.namespace
				-	name:	SECRET_VAR
					valueFrom:
						secretKeyRef:
							key: "secret-var"
							name: "secrets"
				-	name: SOME_VAR
					value: "simple string"`,
		},
		{
			desc:  "string with newline",
			input: `hello\nworld`,
			expected: `---
				-	name: ALL_VAR
					value: placeholder
				-	name: KUBERNETES_NAMESPACE
					valueFrom:
						fieldRef:
							fieldPath: metadata.namespace
				-	name:	SECRET_VAR
					valueFrom:
						secretKeyRef:
							key: "secret-var"
							name: "secrets"
				-	name: SOME_VAR
					value: "hello\nworld"`,
		},
	}

	for _, sample := range samples {
		defaults := map[string]string{
			"SOME_VAR":   sample.input.(string),
			"ALL_VAR":    "placeholder",
			"SECRET_VAR": "the-secret",
		}

		vars, err := getEnvVars(role, ExportSettings{Defaults: defaults})
		sample.check(t, vars, err)
	}
}

func TestPodGetEnvVarsFromConfigSizingCountKube(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SIZING_FOO_COUNT",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
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

	actual, err := testhelpers.RoundtripNode(ev, nil)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigSizingCountHelm(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SIZING_FOO_COUNT",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})

	config := map[string]interface{}{
		"Values.sizing.foo.count": "22",
	}

	actual, err := testhelpers.RoundtripNode(ev, config)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigSizingPortsKube(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MIN",
		},
		&model.ConfigurationVariable{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MAX",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
					Run: &model.RoleRun{
						ExposedPorts: []*model.RoleRunExposedPort{
							&model.RoleRunExposedPort{
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
	})

	actual, err := testhelpers.RoundtripNode(ev, nil)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigSizingPortsHelm(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MIN",
		},
		&model.ConfigurationVariable{
			Name: "KUBE_SIZING_FOO_PORTS_STORE_MAX",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
					Run: &model.RoleRun{
						ExposedPorts: []*model.RoleRunExposedPort{
							&model.RoleRunExposedPort{
								Name:                "store",
								CountIsConfigurable: true,
								InternalPort:        333,
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

	actual, err := testhelpers.RoundtripNode(ev, config)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationCounterKube(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SECRETS_GENERATION_COUNTER",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})

	actual, err := testhelpers.RoundtripNode(ev, nil)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationCounterHelm(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SECRETS_GENERATION_COUNTER",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})

	config := map[string]interface{}{
		"Values.kube.secrets_generation_counter": "3",
	}

	actual, err := testhelpers.RoundtripNode(ev, config)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationNameKube(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SECRETS_GENERATION_NAME",
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})

	actual, err := testhelpers.RoundtripNode(ev, nil)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigGenerationNameHelm(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "KUBE_SECRETS_GENERATION_NAME",
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})

	config := map[string]interface{}{
		"Chart.Version":                          "CV",
		"Values.kube.secrets_generation_counter": "SGC",
	}

	actual, err := testhelpers.RoundtripNode(ev, config)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigSecretsKube(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name:   "A_SECRET",
			Secret: true,
		},
	}, ExportSettings{
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})

	actual, err := testhelpers.RoundtripNode(ev, nil)
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
	`, actual)
}

func TestPodGetEnvVarsFromConfigSecretsHelm(t *testing.T) {
	assert := assert.New(t)

	settings := ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	}

	t.Run("Plain", func(t *testing.T) {
		ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
			&model.ConfigurationVariable{
				Name:   "A_SECRET",
				Secret: true,
			},
		}, settings)
		if !assert.NoError(err) {
			return
		}

		actual, err := testhelpers.RoundtripNode(ev, nil)
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
		`, actual)
	})

	t.Run("Generated", func(t *testing.T) {
		cv := []*model.ConfigurationVariable{
			&model.ConfigurationVariable{
				Name:   "A_SECRET",
				Secret: true,
				Generator: &model.ConfigurationVariableGenerator{
					ID:        "no",
					Type:      model.GeneratorTypePassword,
					ValueType: "foo-login",
				},
			},
		}

		config := map[string]interface{}{
			"Chart.Version":                          "CV",
			"Values.kube.secrets_generation_counter": "SGC",
		}

		ev, err := getEnvVarsFromConfigs(cv, settings)
		if !assert.NoError(err) {
			return
		}

		t.Run("AsIs", func(t *testing.T) {
			actual, err := testhelpers.RoundtripNode(ev, config)
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
			`, actual)
		})

		t.Run("Overidden", func(t *testing.T) {
			config := map[string]interface{}{
				"Values.secrets.A_SECRET": "user's choice",
			}

			actual, err := testhelpers.RoundtripNode(ev, config)
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
			`, actual)
		})

		cv[0].Immutable = true
		ev, err = getEnvVarsFromConfigs(cv, settings)
		if !assert.NoError(err) {
			return
		}

		t.Run("Immutable", func(t *testing.T) {
			actual, err := testhelpers.RoundtripNode(ev, config)
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
			`, actual)
		})
	})
}

func TestPodGetEnvVarsFromConfigNonSecretKube(t *testing.T) {
	assert := assert.New(t)

	settings := ExportSettings{
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	}

	t.Run("Present", func(t *testing.T) {
		ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
			&model.ConfigurationVariable{
				Name:    "SOMETHING",
				Default: "or other",
			},
		}, settings)

		actual, err := testhelpers.RoundtripNode(ev, nil)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: "or other"
		`, actual)
	})

	t.Run("Missing", func(t *testing.T) {
		ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
			&model.ConfigurationVariable{
				Name: "SOMETHING",
			},
		}, settings)

		actual, err := testhelpers.RoundtripNode(ev, nil)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
		`, actual)
	})
}

func TestPodGetEnvVarsFromConfigNonSecretHelmUserOptional(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name: "SOMETHING",
			Type: model.CVTypeUser,
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})
	if !assert.NoError(err) {
		return
	}

	t.Run("Missing", func(t *testing.T) {
		actual, err := testhelpers.RoundtripNode(ev, nil)
		if !assert.NoError(err) {
			return
		}

		testhelpers.IsYAMLEqualString(assert, `---
			-	name: "KUBERNETES_NAMESPACE"
				valueFrom:
					fieldRef:
						fieldPath: "metadata.namespace"
			-	name: "SOMETHING"
				value: 
		`, actual)
	})

	t.Run("Present", func(t *testing.T) {
		config := map[string]interface{}{
			"Values.env.SOMETHING": "else",
		}

		actual, err := testhelpers.RoundtripNode(ev, config)
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
		`, actual)
	})
}

func TestPodGetEnvVarsFromConfigNonSecretHelmUserRequired(t *testing.T) {
	assert := assert.New(t)

	ev, err := getEnvVarsFromConfigs([]*model.ConfigurationVariable{
		&model.ConfigurationVariable{
			Name:     "SOMETHING",
			Type:     model.CVTypeUser,
			Required: true,
		},
	}, ExportSettings{
		CreateHelmChart: true,
		RoleManifest: &model.RoleManifest{
			Roles: []*model.Role{
				&model.Role{
					Name: "foo",
				},
			},
		},
	})

	t.Run("Missing", func(t *testing.T) {
		_, err = testhelpers.RenderNode(ev, nil)
		assert.EqualError(err,
			`template: :7:12: executing "" at <required "SOMETHING ...>: error calling required: SOMETHING configuration missing`)
	})

	t.Run("Present", func(t *testing.T) {
		config := map[string]interface{}{
			"Values.env.SOMETHING": "needed",
		}

		actual, err := testhelpers.RoundtripNode(ev, config)
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
		`, actual)
	})
}

func TestPodGetContainerLivenessProbe(t *testing.T) {
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
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	samples := []Sample{
		{
			desc:     "No probe",
			input:    nil,
			expected: `---`,
		},
		{
			desc: "Port probe",
			input: &model.HealthProbe{
				Port: 1234,
			},
			expected: `---
				tcpSocket:
					port: 1234`,
		},
		{
			desc: "Command probe",
			input: &model.HealthProbe{
				Command: []string{"rm", "-rf", "--no-preserve-root", "/"},
			},
			expected: `---
				exec:
					command: [ rm, "-rf", "--no-preserve-root", /]`,
		},
		{
			desc: "URL probe (simple)",
			input: &model.HealthProbe{
				URL: "http://example.com/path",
			},
			expected: `---
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
			expected: `---
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
			err: "Invalid readiness URL health check for myrole: parse ://: missing protocol scheme",
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
				httpGet:
					scheme: HTTP
					port:   80
					path:   "/path"`,
		},
		{
			desc: "Port probe, readiness timeout",
			input: &model.HealthProbe{
				Port:    1234,
				Timeout: 20,
			},
			expected: `---
				timeoutSeconds: 20
				tcpSocket:
					port: 1234`,
		},
		{
			desc: "Command probe, readiness timeout",
			input: &model.HealthProbe{
				Command: []string{"rm", "-rf", "--no-preserve-root", "/"},
				Timeout: 20,
			},
			expected: `---
				timeoutSeconds: 20
				exec:
					command: [ rm, "-rf", "--no-preserve-root", /]`,
		},
		{
			desc: "URL probe (simple), readiness timeout",
			input: &model.HealthProbe{
				URL:     "http://example.com/path",
				Timeout: 20,
			},
			expected: `---
				timeoutSeconds: 20
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
			desc: "Success Threshold",
			input: &model.HealthProbe{
				SuccessThreshold: 20,
				Port:             2289,
			},
			expected: `---
				successThreshold: 20
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
				failureThreshold: 20
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
				periodSeconds: 20
				tcpSocket:
					port: 2289`,
		},
	}

	for _, sample := range samples {
		probe, _ := sample.input.(*model.HealthProbe)
		role.Run.HealthCheck = &model.HealthCheck{Readiness: probe}
		actual, err := getContainerReadinessProbe(role)
		sample.check(t, actual, err)
	}
}

func podTestLoadRoleFrom(assert *assert.Assertions, roleName, manifestName string) *model.Role {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests", manifestName)
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")

	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return nil
	}
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release}, nil)
	if !assert.NoError(err) {
		return nil
	}
	role := manifest.LookupRole(roleName)
	if !assert.NotNil(role, "Failed to find role %s", roleName) {
		return nil
	}

	return role
}

func podTestLoadRole(assert *assert.Assertions, roleName string) *model.Role {
	return podTestLoadRoleFrom(assert, roleName, "pods.yml")
}

func TestPodPreFlightKube(t *testing.T) {
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

	actual, err := testhelpers.RoundtripNode(pod, nil)
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
		"Values.kube.registry.hostname":         "R",
		"Values.kube.organization":              "O",
		"Values.env.KUBE_SERVICE_DOMAIN_SUFFIX": "KSDS",
	}

	actual, err := testhelpers.RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "KUBE_SERVICE_DOMAIN_SUFFIX"
					value: "KSDS"
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
				securityContext: ~
				volumeMounts: ~
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes: ~
	`, actual)
}

func TestPodPostFlightKube(t *testing.T) {
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

	actual, err := testhelpers.RoundtripNode(pod, nil)
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
		"Values.kube.registry.hostname":         "R",
		"Values.kube.organization":              "O",
		"Values.env.KUBE_SERVICE_DOMAIN_SUFFIX": "KSDS",
	}

	actual, err := testhelpers.RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "post-role"
			labels:
				skiff-role-name: "post-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "KUBE_SERVICE_DOMAIN_SUFFIX"
					value: "KSDS"
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
				securityContext: ~
				volumeMounts: ~
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes: ~
	`, actual)
}

func TestPodMemoryKube(t *testing.T) {
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

	actual, err := testhelpers.RoundtripNode(pod, nil)
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
		"Values.kube.registry.hostname":         "R",
		"Values.kube.organization":              "O",
		"Values.env.KUBE_SERVICE_DOMAIN_SUFFIX": "KSDS",
	}

	actual, err := testhelpers.RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "KUBE_SERVICE_DOMAIN_SUFFIX"
					value: "KSDS"
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
				securityContext: ~
				volumeMounts: ~
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes: ~
	`, actual)
}

func TestPodMemoryHelmActive(t *testing.T) {
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
		"Values.kube.registry.hostname":         "R",
		"Values.kube.organization":              "O",
		"Values.env.KUBE_SERVICE_DOMAIN_SUFFIX": "KSDS",
		"Values.sizing.memory.requests":         "true",
		"Values.sizing.pre_role.memory.request": "1",
		"Values.sizing.memory.limits":           "true",
		"Values.sizing.pre_role.memory.limit":   "10",
	}

	actual, err := testhelpers.RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "KUBE_SERVICE_DOMAIN_SUFFIX"
					value: "KSDS"
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
				securityContext: ~
				volumeMounts: ~
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes: ~
	`, actual)
}

func TestPodCPUKube(t *testing.T) {
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

	actual, err := testhelpers.RoundtripNode(pod, nil)
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
		"Values.kube.registry.hostname":         "R",
		"Values.kube.organization":              "O",
		"Values.env.KUBE_SERVICE_DOMAIN_SUFFIX": "KSDS",
	}

	actual, err := testhelpers.RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "KUBE_SERVICE_DOMAIN_SUFFIX"
					value: "KSDS"
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
				securityContext: ~
				volumeMounts: ~
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes: ~
	`, actual)
}

func TestPodCPUAHelmActive(t *testing.T) {
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
		"Values.kube.registry.hostname":         "R",
		"Values.kube.organization":              "O",
		"Values.env.KUBE_SERVICE_DOMAIN_SUFFIX": "KSDS",

		"Values.sizing.cpu.requests":         "true",
		"Values.sizing.pre_role.cpu.request": "1",
		"Values.sizing.cpu.limits":           "true",
		"Values.sizing.pre_role.cpu.limit":   "10",
	}

	actual, err := testhelpers.RoundtripNode(pod, config)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		apiVersion: "v1"
		kind: "Pod"
		metadata:
			name: "pre-role"
			labels:
				skiff-role-name: "pre-role"
		spec:
			containers:
			-	env:
				-	name: "KUBERNETES_NAMESPACE"
					valueFrom:
						fieldRef:
							fieldPath: "metadata.namespace"
				-	name: "KUBE_SERVICE_DOMAIN_SUFFIX"
					value: "KSDS"
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
				securityContext: ~
				volumeMounts: ~
			dnsPolicy: "ClusterFirst"
			imagePullSecrets:
			-	name: "registry-credentials"
			restartPolicy: "OnFailure"
			terminationGracePeriodSeconds: 600
			volumes: ~
	`, actual)
}

func TestGetSecurityContext(t *testing.T) {
	assert := assert.New(t)

	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	sc := getSecurityContext(role)
	if !assert.NotNil(sc) {
		return
	}

	actual, err := testhelpers.RoundtripNode(sc, nil)
	if !assert.NoError(err) {
		return
	}
	testhelpers.IsYAMLEqualString(assert, `---
		capabilities:
			add:
			-	"SOMETHING"
	`, actual)
}

func TestPodGetContainerImageNameKube(t *testing.T) {
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

	actual, err := testhelpers.RoundtripNode(nameNode, config)
	if !assert.NoError(err) {
		return
	}

	testhelpers.IsYAMLEqualString(assert, `---
		R/O/theRepo-myrole:d0aca33ba5bc55dce697d9d57b46e1b23688659c
	`, actual)
}

func TestPodGetContainerPortsKube(t *testing.T) {
	assert := assert.New(t)
	role := podTestLoadRoleFrom(assert, "myrole", "exposed-ports.yml")
	if role == nil {
		return
	}

	settings := ExportSettings{}

	ports, err := getContainerPorts(role, settings)
	assert.Nil(err)
	assert.NotNil(ports)

	actual, err := testhelpers.RoundtripNode(ports, nil)
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

	actual, err := testhelpers.RoundtripNode(ports, nil)
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

	actual, err := testhelpers.RoundtripNode(ports, config)
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
	assert := assert.New(t)

	sv := makeSecretVar("foo", false)

	actual, err := testhelpers.RoundtripNode(sv, nil)
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
	assert := assert.New(t)

	sv := makeSecretVar("foo", true)

	config := map[string]interface{}{
		"Chart.Version":                          "CV",
		"Values.kube.secrets_generation_counter": "SGC",
	}

	actual, err := testhelpers.RoundtripNode(sv, config)
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
