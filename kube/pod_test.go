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

func TestPodGetVolumes(t *testing.T) {
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	claims := getAllVolumeClaims(role, false)
	assert.Len(claims, 2, "expected two claims")

	var persistentClaim, sharedClaim helm.Node
	for _, claim := range claims {
		switch claim.Get("metadata", "name").String() {
		case role.Run.PersistentVolumes[0].Tag:
			persistentClaim = claim
		case role.Run.SharedVolumes[0].Tag:
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

func TestPodGetVolumeMounts(t *testing.T) {
	assert := assert.New(t)
	role := podTemplateTestLoadRole(assert)
	if role == nil {
		return
	}

	volumeMounts := getVolumeMounts(role).Values()
	assert.Len(volumeMounts, 2)

	var persistentMount, sharedMount helm.Node
	for _, mount := range volumeMounts {
		switch mount.Get("name").String() {
		case "persistent-volume":
			persistentMount = mount
		case "shared-volume":
			sharedMount = mount
		default:
			assert.Fail("Got unexpected volume mount", "%+v", mount)
		}
	}
	assert.Equal("/mnt/persistent", persistentMount.Get("mountPath").String())
	assert.Equal("false", persistentMount.Get("readOnly").String())
	assert.Equal("/mnt/shared", sharedMount.Get("mountPath").String())
	assert.Equal("false", sharedMount.Get("readOnly").String())
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

func podTestLoadRole(assert *assert.Assertions, roleName string) *model.Role {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/pods.yml")
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

func TestPodPreFlight(t *testing.T) {
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

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(pod); !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}

	expectedYAML := strings.Replace(`---
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
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}

	testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestPodPostFlight(t *testing.T) {
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

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(pod); !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}

	expectedYAML := strings.Replace(`---
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
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}
	testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestPodMemory(t *testing.T) {
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

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(pod); !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}

	expectedYAML := strings.Replace(`---
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
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}

	testhelpers.IsYAMLSubset(assert, expected, actual)
}

func TestPodCPU(t *testing.T) {
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

	yamlConfig := &bytes.Buffer{}
	if err := helm.NewEncoder(yamlConfig).Encode(pod); !assert.NoError(err) {
		return
	}

	var expected, actual interface{}
	if !assert.NoError(yaml.Unmarshal(yamlConfig.Bytes(), &actual)) {
		return
	}

	expectedYAML := strings.Replace(`---
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
	`, "\t", "    ", -1)
	if !assert.NoError(yaml.Unmarshal([]byte(expectedYAML), &expected)) {
		return
	}

	testhelpers.IsYAMLSubset(assert, expected, actual)
}
