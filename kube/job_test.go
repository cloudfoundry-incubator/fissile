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

func jobTestLoadRole(assert *assert.Assertions, roleName, manifestName string) *model.Role {
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

func TestJobPreFlight(t *testing.T) {
	assert := assert.New(t)
	role := jobTestLoadRole(assert, "pre-role", "jobs.yml")
	if role == nil {
		return
	}

	job, err := NewJob(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create job from role pre-role") {
		return
	}
	assert.NotNil(job)

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(job); !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}

	expectedYAML := strings.Replace(`---
		apiVersion: extensions/v1beta1
		kind: Job
		metadata:
			name: pre-role
		spec:
			template:
				metadata:
					name: pre-role
				spec:
					containers:
					-
						name: pre-role
					restartPolicy: OnFailure
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestJobPostFlight(t *testing.T) {
	assert := assert.New(t)
	role := jobTestLoadRole(assert, "post-role", "jobs.yml")
	if role == nil {
		return
	}

	job, err := NewJob(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create job from role post-role") {
		return
	}
	assert.NotNil(job)

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(job); !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}
	expectedYAML := strings.Replace(`---
		apiVersion: extensions/v1beta1
		kind: Job
		metadata:
			name: post-role
		spec:
			template:
				metadata:
					name: post-role
				spec:
					containers:
					-
						name: post-role
					restartPolicy: OnFailure
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestJobWithAnnotations(t *testing.T) {
	assert := assert.New(t)

	role := jobTestLoadRole(assert, "role", "job-with-annotation.yml")
	if role == nil {
		return
	}

	job, err := NewJob(role, ExportSettings{
		Opinions: model.NewEmptyOpinions(),
	}, nil)
	if !assert.NoError(err, "Failed to create job from role pre-role") {
		return
	}
	assert.NotNil(job)

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(job); !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}
	expectedYAML := strings.Replace(`---
		apiVersion: extensions/v1beta1
		kind: Job
		metadata:
			name: role
			annotations:
				helm.sh/hook: post-install
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}

	testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestJobHelmWithDefaults(t *testing.T) {
	assert := assert.New(t)
	role := jobTestLoadRole(assert, "pre-role", "jobs.yml")
	if role == nil {
		return
	}

	job, err := NewJob(role, ExportSettings{
		Opinions:        model.NewEmptyOpinions(),
		CreateHelmChart: true,
	}, nil)
	if !assert.NoError(err, "Failed to create job from role pre-role") {
		return
	}
	assert.NotNil(job)

	// Render should fail due to the template referencing a number
	// of variables which must be non-nil to work.
	_, err = testhelpers.RenderNode(job, nil)
	assert.EqualError(err, `template: :5:155: executing "" at <trimSuffix>: wrong number of args for trimSuffix: want 2 got 1`)
}

func TestJobHelmFilledKube15(t *testing.T) {
	assert := assert.New(t)
	role := jobTestLoadRole(assert, "pre-role", "jobs.yml")
	if role == nil {
		return
	}

	job, err := NewJob(role, ExportSettings{
		Opinions:        model.NewEmptyOpinions(),
		CreateHelmChart: true,
		Repository:      "the_repos",
	}, nil)
	if !assert.NoError(err, "Failed to create job from role pre-role") {
		return
	}
	assert.NotNil(job)

	workDir, err := os.Getwd()
	assert.NoError(err)
	fakeTemplateDir := filepath.Join(workDir, "../test-assets/fake-templates")

	config := map[string]interface{}{
		"Capabilities.KubeVersion.Major": "1",
		"Capabilities.KubeVersion.Minor": "5",
		// Fake location for a fake `secrets.yaml`.
		"Template.BasePath": fakeTemplateDir,
	}

	jobYAML, err := testhelpers.RenderNode(job, config)
	if !assert.NoError(err) {
		return
	}

	// The various <no value> seen below come from, in order:
	// - Release.Revision
	// - Values.kube.registry.hostname
	// - Values.kube.organization
	// none of which are defined in the config.
	//
	// Another undefined variable,
	// Values.env.KUBE_SERVICE_DOMAIN_SUFFIX, yields an empty
	// field, see the value of KUBE_SERVICE_DOMAIN_SUFFIX.

	expectedJobYAML := `---
# The pre-role role contains the following jobs:
#
# new_hostname
apiVersion: extensions/v1beta1
kind: "Job"
metadata:
  name: "pre-role-<no value>"
spec:
  template:
    metadata:
      name: "pre-role"
      labels:
        skiff-role-name: "pre-role"
      annotations:
        checksum/config: 08c80ed11902eefef09739d41c91408238bb8b5e7be7cc1e5db933b7c8de65c3
    spec:
      containers:
      - env:
        - name: "KUBERNETES_NAMESPACE"
          valueFrom:
            fieldRef:
              fieldPath: "metadata.namespace"
        - name: "KUBE_SERVICE_DOMAIN_SUFFIX"
          value: 
        image: "<no value>/<no value>/the_repos-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
        lifecycle:
          preStop:
            exec:
              command:
              - "/opt/fissile/pre-stop.sh"
        livenessProbe: ~
        name: "pre-role"
        ports: ~
        readinessProbe: ~
        resources: ~
        securityContext: ~
        volumeMounts: ~
      dnsPolicy: "ClusterFirst"
      imagePullSecrets:
      - name: "registry-credentials"
      restartPolicy: "OnFailure"
      terminationGracePeriodSeconds: 600
      volumes: ~
`
	assert.Equal(expectedJobYAML, string(jobYAML))
}

func TestJobHelmFilledKube16(t *testing.T) {
	assert := assert.New(t)
	role := jobTestLoadRole(assert, "pre-role", "jobs.yml")
	if role == nil {
		return
	}

	job, err := NewJob(role, ExportSettings{
		Opinions:        model.NewEmptyOpinions(),
		CreateHelmChart: true,
		Repository:      "the_repos",
	}, nil)
	if !assert.NoError(err, "Failed to create job from role pre-role") {
		return
	}
	assert.NotNil(job)

	workDir, err := os.Getwd()
	assert.NoError(err)
	fakeTemplateDir := filepath.Join(workDir, "../test-assets/fake-templates")

	config := map[string]interface{}{
		"Capabilities.KubeVersion.Major": "1",
		"Capabilities.KubeVersion.Minor": "6",
		// Fake location for a fake `secrets.yaml`.
		"Template.BasePath":                     fakeTemplateDir,
		"Release.Revision":                      "42",
		"Values.kube.registry.hostname":         "docker.suse.fake",
		"Values.kube.organization":              "splat",
		"Values.env.KUBE_SERVICE_DOMAIN_SUFFIX": "domestic",
	}

	jobYAML, err := testhelpers.RenderNode(job, config)
	if !assert.NoError(err) {
		return
	}

	expectedJobYAML := `---
# The pre-role role contains the following jobs:
#
# new_hostname
apiVersion: batch/v1
kind: "Job"
metadata:
  name: "pre-role-42"
spec:
  template:
    metadata:
      name: "pre-role"
      labels:
        skiff-role-name: "pre-role"
      annotations:
        checksum/config: 08c80ed11902eefef09739d41c91408238bb8b5e7be7cc1e5db933b7c8de65c3
    spec:
      containers:
      - env:
        - name: "KUBERNETES_NAMESPACE"
          valueFrom:
            fieldRef:
              fieldPath: "metadata.namespace"
        - name: "KUBE_SERVICE_DOMAIN_SUFFIX"
          value: "domestic"
        image: "docker.suse.fake/splat/the_repos-pre-role:b0668a0daba46290566d99ee97d7b45911a53293"
        lifecycle:
          preStop:
            exec:
              command:
              - "/opt/fissile/pre-stop.sh"
        livenessProbe: ~
        name: "pre-role"
        ports: ~
        readinessProbe: ~
        resources: ~
        securityContext: ~
        volumeMounts: ~
      dnsPolicy: "ClusterFirst"
      imagePullSecrets:
      - name: "registry-credentials"
      restartPolicy: "OnFailure"
      terminationGracePeriodSeconds: 600
      volumes: ~
`
	assert.Equal(expectedJobYAML, string(jobYAML))
}
