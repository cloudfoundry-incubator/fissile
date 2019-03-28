package kube

import (
	"os"
	"path/filepath"
	"testing"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/loader"
	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/stretchr/testify/assert"
)

func deploymentTestLoad(assert *assert.Assertions, roleName, manifestName string) *model.InstanceGroup {
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

	instanceGroup := manifest.LookupInstanceGroup(roleName)
	if !assert.NotNil(instanceGroup, "Failed to find instance group %s", roleName) {
		return nil
	}
	return instanceGroup
}

type FakeGrapher struct {
}

func (f FakeGrapher) GraphNode(nodeName string, attrs map[string]string) error {
	return nil
}

func (f FakeGrapher) GraphEdge(fromNode, toNode string, attrs map[string]string) error {
	return nil
}

func TestNewDeploymentKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	instanceGroup := deploymentTestLoad(assert, "some-group", "pod-with-valid-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	settings := ExportSettings{}

	grapher := FakeGrapher{}

	deployment, svc, err := NewDeployment(instanceGroup, settings, grapher)

	assert.NoError(err)
	assert.Nil(svc)
	assert.NotNil(deployment)
	assert.Equal(deployment.Get("kind").String(), "Deployment")
	assert.Equal(deployment.Get("metadata", "name").String(), "some-group")
}

func TestNewDeploymentHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	instanceGroup := deploymentTestLoad(assert, "some-group", "pod-with-valid-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	settings := ExportSettings{
		CreateHelmChart: true,
		Repository:      "the_repos",
	}

	grapher := FakeGrapher{}

	deployment, svc, err := NewDeployment(instanceGroup, settings, grapher)

	assert.NoError(err)
	assert.Nil(svc)
	assert.NotNil(deployment)
	assert.Equal(deployment.Get("kind").String(), "Deployment")
	assert.Equal(deployment.Get("metadata", "name").String(), "some-group")

	t.Run("Defaults", func(t *testing.T) {
		t.Parallel()
		// Rendering fails with defaults, template needs information
		// about sizing and the like.
		config := map[string]interface{}{
			"Values.sizing.some_group.count": nil,
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :11:17: executing "" at <fail "some_group must have at least 1 instances">: error calling fail: some_group must have at least 1 instances`)
	})

	t.Run("Configured, not enough replicas", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.some_group.count":                 "0",
			"Values.sizing.some_group.affinity.nodeAffinity": "snafu",
			"Values.sizing.some_group.capabilities":          []interface{}{},
			"Values.kube.registry.hostname":                  "docker.suse.fake",
			"Values.kube.organization":                       "splat",
			"Values.env.KUBERNETES_CLUSTER_DOMAIN":           "cluster.local",
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :11:17: executing "" at <fail "some_group must have at least 1 instances">: error calling fail: some_group must have at least 1 instances`)
	})

	t.Run("Configured, too many replicas", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.some_group.count":                 "10",
			"Values.sizing.some_group.affinity.nodeAffinity": "snafu",
			"Values.sizing.some_group.capabilities":          []interface{}{},
			"Values.kube.registry.hostname":                  "docker.suse.fake",
			"Values.kube.organization":                       "splat",
			"Values.env.KUBERNETES_CLUSTER_DOMAIN":           "cluster.local",
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :7:17: executing "" at <fail "some_group cannot have more than 1 instances">: error calling fail: some_group cannot have more than 1 instances`)
	})

	t.Run("Configured, bad key sizing.HA", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.HA":               "true",
			"Values.sizing.some_group.count": "1",
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :15:21: executing "" at <fail "Bad use of moved variable sizing.HA. The new name to use is config.HA">: error calling fail: Bad use of moved variable sizing.HA. The new name to use is config.HA`)
	})

	t.Run("Configured, bad key sizing.memory.limits", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.memory.limits":    "true",
			"Values.sizing.some_group.count": "1",
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :27:70: executing "" at <fail "Bad use of moved variable sizing.memory.limits. The new name to use is config.memory.limits">: error calling fail: Bad use of moved variable sizing.memory.limits. The new name to use is config.memory.limits`)
	})

	t.Run("Configured, bad key sizing.memory.requests", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.memory.requests":  "true",
			"Values.sizing.some_group.count": "1",
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :31:74: executing "" at <fail "Bad use of moved variable sizing.memory.requests. The new name to use is config.memory.requests">: error calling fail: Bad use of moved variable sizing.memory.requests. The new name to use is config.memory.requests`)
	})

	t.Run("Configured, bad key sizing.cpu.limits", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.cpu.limits":       "true",
			"Values.sizing.some_group.count": "1",
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :19:64: executing "" at <fail "Bad use of moved variable sizing.cpu.limits. The new name to use is config.cpu.limits">: error calling fail: Bad use of moved variable sizing.cpu.limits. The new name to use is config.cpu.limits`)
	})

	t.Run("Configured, bad key sizing.cpu.requests", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.cpu.requests":     "true",
			"Values.sizing.some_group.count": "1",
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :23:68: executing "" at <fail "Bad use of moved variable sizing.cpu.requests. The new name to use is config.cpu.requests">: error calling fail: Bad use of moved variable sizing.cpu.requests. The new name to use is config.cpu.requests`)
	})

	t.Run("Configured", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.config.use_istio":                        true,
			"Values.sizing.some_group.count":                 "1",
			"Values.sizing.some_group.affinity.nodeAffinity": "snafu",
			"Values.sizing.some_group.capabilities":          []interface{}{},
			"Values.kube.registry.hostname":                  "docker.suse.fake",
			"Values.kube.organization":                       "splat",
			"Values.env.KUBERNETES_CLUSTER_DOMAIN":           "cluster.local",
		}

		actual, err := RoundtripNode(deployment, config)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "extensions/v1beta1"
			kind: "Deployment"
			metadata:
				name: "some-group"
				labels:
					app: "some-group"
					app.kubernetes.io/component: some-group
					app.kubernetes.io/instance: MyRelease
					app.kubernetes.io/managed-by: Tiller
					app.kubernetes.io/name: MyChart
					app.kubernetes.io/version: 1.22.333.4444
					helm.sh/chart: MyChart-42.1_foo
					skiff-role-name: "some-group"
					version: 1.22.333.4444
			spec:
				replicas: 1
				selector:
					matchLabels:
						skiff-role-name: "some-group"
				template:
					metadata:
						name: "some-group"
						labels:
							app: "some-group"
							app.kubernetes.io/component: some-group
							app.kubernetes.io/instance: MyRelease
							app.kubernetes.io/managed-by: Tiller
							app.kubernetes.io/name: MyChart
							app.kubernetes.io/version: 1.22.333.4444
							helm.sh/chart: MyChart-42.1_foo
							skiff-role-name: "some-group"
							version: 1.22.333.4444
						annotations:
							checksum/config: 08c80ed11902eefef09739d41c91408238bb8b5e7be7cc1e5db933b7c8de65c3
							sidecar.istio.io/inject: "false"
					spec:
						affinity:
							podAntiAffinity:
								preferredDuringSchedulingIgnoredDuringExecution:
								-	podAffinityTerm:
										labelSelector:
											matchExpressions:
											-	key: "app.kubernetes.io/component"
												operator: "In"
												values:
												-	"some-group"
										topologyKey: "beta.kubernetes.io/os"
									weight: 100
							nodeAffinity: "snafu"
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
							image: "docker.suse.fake/splat/the_repos-some-group:3b960ef56f837ae186cdd546d03750cca62676bc"
							lifecycle:
								preStop:
									exec:
										command:
										-	"/opt/fissile/pre-stop.sh"
							livenessProbe: ~
							name: "some-group"
							ports: ~
							readinessProbe:
								exec:
									command: [ /opt/fissile/readiness-probe.sh ]
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
						- name: "registry-credentials"
						restartPolicy: "Always"
						terminationGracePeriodSeconds: 600
						volumes:
						-	name: deployment-manifest
							secret:
								items:
								-	key: deployment-manifest
									path: deployment-manifest.yml
								secretName: deployment-manifest
		`, actual)
	})
}

func TestNewDeploymentIstioManagedHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	instanceGroup := deploymentTestLoad(assert, "istio-managed-group", "pod-with-valid-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	settings := ExportSettings{
		CreateHelmChart: true,
		Repository:      "the_repos",
	}

	grapher := FakeGrapher{}

	deployment, svc, err := NewDeployment(instanceGroup, settings, grapher)

	assert.NoError(err)
	assert.Nil(svc)
	assert.NotNil(deployment)
	assert.Equal(deployment.Get("kind").String(), "Deployment")
	assert.Equal(deployment.Get("metadata", "name").String(), "istio-managed-group")

	t.Run("Configured", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.config.use_istio":                                 "true",
			"Values.sizing.istio_managed_group.count":                 "1",
			"Values.sizing.istio_managed_group.affinity.nodeAffinity": "snafu",
			"Values.sizing.istio_managed_group.capabilities":          []interface{}{},
			"Values.kube.registry.hostname":                           "docker.suse.fake",
			"Values.kube.organization":                                "splat",
			"Values.env.KUBERNETES_CLUSTER_DOMAIN":                    "cluster.local",
		}

		actual, err := RoundtripNode(deployment, config)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "extensions/v1beta1"
			kind: "Deployment"
			metadata:
				name: "istio-managed-group"
				labels:
					app: "istio-managed-group"
					app.kubernetes.io/component: istio-managed-group
					app.kubernetes.io/instance: MyRelease
					app.kubernetes.io/managed-by: Tiller
					app.kubernetes.io/name: MyChart
					app.kubernetes.io/version: 1.22.333.4444
					helm.sh/chart: MyChart-42.1_foo
					skiff-role-name: "istio-managed-group"
					version: 1.22.333.4444
			spec:
				replicas: 1
				selector:
					matchLabels:
						app: "istio-managed-group"
						skiff-role-name: "istio-managed-group"
						version: 1.22.333.4444
				template:
					metadata:
						name: "istio-managed-group"
						labels:
							app: "istio-managed-group"
							app.kubernetes.io/component: istio-managed-group
							app.kubernetes.io/instance: MyRelease
							app.kubernetes.io/managed-by: Tiller
							app.kubernetes.io/name: MyChart
							app.kubernetes.io/version: 1.22.333.4444
							helm.sh/chart: MyChart-42.1_foo
							skiff-role-name: "istio-managed-group"
							version: 1.22.333.4444
						annotations:
							checksum/config: 08c80ed11902eefef09739d41c91408238bb8b5e7be7cc1e5db933b7c8de65c3
					spec:
						affinity:
							podAntiAffinity:
								preferredDuringSchedulingIgnoredDuringExecution:
								-	podAffinityTerm:
										labelSelector:
											matchExpressions:
											-	key: "app.kubernetes.io/component"
												operator: "In"
												values:
												-	"istio-managed-group"
										topologyKey: "beta.kubernetes.io/os"
									weight: 100
							nodeAffinity: "snafu"
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
							image: "docker.suse.fake/splat/the_repos-istio-managed-group:3b960ef56f837ae186cdd546d03750cca62676bc"
							lifecycle:
								preStop:
									exec:
										command:
										-	"/opt/fissile/pre-stop.sh"
							livenessProbe: ~
							name: "istio-managed-group"
							ports: ~
							readinessProbe:
								exec:
									command: [ /opt/fissile/readiness-probe.sh ]
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
						- name: "registry-credentials"
						restartPolicy: "Always"
						terminationGracePeriodSeconds: 600
						volumes:
						-	name: deployment-manifest
							secret:
								items:
								-	key: deployment-manifest
									path: deployment-manifest.yml
								secretName: deployment-manifest
		`, actual)
	})
}

func TestGetAffinityBlock(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	instanceGroup := deploymentTestLoad(assert, "some-group", "pod-with-valid-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	affinity := getAffinityBlock(instanceGroup)

	assert.NotNil(affinity.Get("podAntiAffinity"))
	assert.NotNil(affinity.Get("nodeAffinity"))
	assert.Equal(affinity.Names(), []string{"podAntiAffinity", "nodeAffinity"})
	assert.Equal(affinity.Get("nodeAffinity").Block(), "if .Values.sizing.some_group.affinity.nodeAffinity")

	instanceGroup = deploymentTestLoad(assert, "some-group", "pod-with-no-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	affinity = getAffinityBlock(instanceGroup)

	assert.Nil(affinity.Get("podAntiAffinity"))
	assert.NotNil(affinity.Get("nodeAffinity"))
	assert.Equal(affinity.Names(), []string{"nodeAffinity"})
	assert.Equal(affinity.Get("nodeAffinity").Block(), "if .Values.sizing.some_group.affinity.nodeAffinity")
}

func createEmptySpec() *helm.Mapping {
	emptySpec := helm.NewMapping()
	template := helm.NewMapping()
	metadata := helm.NewMapping()
	annotations := helm.NewMapping()
	podspec := helm.NewMapping()

	metadata.Add("annotations", annotations)
	template.Add("metadata", metadata)
	template.Add("spec", podspec)
	emptySpec.Add("template", template)

	return emptySpec
}

func TestAddAffinityRules(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	emptySpec := createEmptySpec()

	//
	// Test instance group with valid anti affinity
	//
	instanceGroup := deploymentTestLoad(assert, "some-group", "pod-with-valid-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	spec := createEmptySpec()

	settings := ExportSettings{CreateHelmChart: true}

	err := addAffinityRules(instanceGroup, spec, settings)

	assert.NotNil(spec.Get("template", "spec", "affinity", "podAntiAffinity"))
	assert.NotNil(spec.Get("template", "spec", "affinity", "nodeAffinity"))
	assert.NoError(err)

	//
	// Test instance group with pod affinity defined
	//
	instanceGroup = deploymentTestLoad(assert, "some-group", "pod-with-invalid-pod-affinity.yml")
	if instanceGroup == nil {
		return
	}

	spec = createEmptySpec()

	err = addAffinityRules(instanceGroup, spec, settings)

	assert.Error(err)
	assert.Equal(spec, emptySpec)

	//
	// Test instance group with node affinity defined
	//
	instanceGroup = deploymentTestLoad(assert, "some-group", "pod-with-invalid-node-affinity.yml")
	if instanceGroup == nil {
		return
	}

	spec = createEmptySpec()

	err = addAffinityRules(instanceGroup, spec, settings)

	assert.Error(err)
	assert.Equal(spec, emptySpec)

	//
	// Test instance group without anti affinity
	//
	instanceGroup = deploymentTestLoad(assert, "some-group", "pod-with-no-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	spec = createEmptySpec()

	err = addAffinityRules(instanceGroup, spec, settings)

	assert.Nil(spec.Get("template", "spec", "affinity", "podAntiAffinity"))
	assert.NotNil(spec.Get("template", "spec", "affinity", "nodeAffinity"))
	assert.NoError(err)

	//
	// Not creating the helm chart should only add the annotation
	//
	instanceGroup = deploymentTestLoad(assert, "some-group", "pod-with-valid-pod-anti-affinity.yml")
	if instanceGroup == nil {
		return
	}

	spec = createEmptySpec()

	settings = ExportSettings{CreateHelmChart: false}

	err = addAffinityRules(instanceGroup, spec, settings)
	assert.Nil(spec.Get("template", "spec", "affinity", "podAntiAffinity"))
	assert.NoError(err)
}

func TestNewDeploymentWithEmptyDirVolume(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	instanceGroup := deploymentTestLoad(assert, "some-group", "colocated-containers-with-deployment-and-empty-dir.yml")
	if instanceGroup == nil {
		return
	}

	settings := ExportSettings{
		CreateHelmChart: true,
		Repository:      "the_repos",
	}

	deployment, svc, err := NewDeployment(instanceGroup, settings, nil)

	assert.NoError(err)
	assert.Nil(svc)
	assert.NotNil(deployment)
	assert.Equal(deployment.Get("kind").String(), "Deployment")
	assert.Equal(deployment.Get("metadata", "name").String(), "some-group")

	t.Run("Defaults", func(t *testing.T) {
		t.Parallel()
		// Rendering fails with defaults, template needs information
		// about sizing and the like.
		config := map[string]interface{}{
			"Values.sizing.some_group.count": nil,
		}
		_, err := RenderNode(deployment, config)
		assert.EqualError(err,
			`template: :11:17: executing "" at <fail "some_group must have at least 1 instances">: error calling fail: some_group must have at least 1 instances`)
	})

	t.Run("Configured", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.sizing.some_group.affinity":     map[string]interface{}{},
			"Values.sizing.some_group.count":        "1",
			"Values.sizing.some_group.capabilities": []interface{}{},
			"Values.sizing.colocated.capabilities":  []interface{}{},
			"Values.kube.registry.hostname":         "docker.suse.fake",
			"Values.kube.organization":              "splat",
			"Values.env.KUBERNETES_CLUSTER_DOMAIN":  "cluster.local",
		}

		actual, err := RoundtripNode(deployment, config)
		if !assert.NoError(err) {
			return
		}
		testhelpers.IsYAMLSubsetString(assert, `---
			apiVersion: "extensions/v1beta1"
			kind: "Deployment"
			spec:
				template:
					spec:
						containers:
						-	name: "some-group"
							volumeMounts:
							-
								name: shared-data
								mountPath: /mnt/shared-data
							-
								mountPath: /opt/fissile/config
								name: deployment-manifest
								readOnly: true
						-	name: "colocated"
							volumeMounts:
							-
								name: shared-data
								mountPath: /mnt/shared-data
							-
								mountPath: /opt/fissile/config
								name: deployment-manifest
								readOnly: true
						volumes:
						-
							name: shared-data
							emptyDir: {}
						-
							name: deployment-manifest
							secret:
								items:
								-	key: deployment-manifest
									path: deployment-manifest.yml
								secretName: deployment-manifest
		`, actual)
	})
}
