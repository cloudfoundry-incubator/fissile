package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"code.cloudfoundry.org/fissile/kube"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/loader"
	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/SUSE/termui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestCleanCacheEmpty(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/no-instance-groups/no-instance-groups.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/ntp-release"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	if assert.NoError(err) {
		err = f.CleanCache()
		assert.Nil(err, "Expected CleanCache to find the release")
	}
}

func TestListPackages(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/no-instance-groups/no-instance-groups.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/bad-release"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(err, "Expected ListPackages to not find the release")

	f.Options.Releases[0] = filepath.Join(workDir, "../test-assets/ntp-release")
	err = f.LoadManifest()
	if assert.NoError(err) {
		err = f.ListPackages()
		assert.Nil(err, "Expected ListPackages to find the release")
	}
}

func TestListJobs(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/no-instance-groups/no-instance-groups.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/bad-release"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(err, "Expected ListJobs to not find the release")

	f.Options.Releases[0] = filepath.Join(workDir, "../test-assets/ntp-release")
	err = f.LoadManifest()
	if assert.NoError(err) {
		err = f.ListJobs()
		assert.Nil(err, "Expected ListJobs to find the release")
	}
}

func TestListProperties(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/no-instance-groups/no-instance-groups.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/bad-release"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	assert.Error(err, "Expected ListProperties to not find the release")

	f.Options.Releases[0] = filepath.Join(workDir, "../test-assets/ntp-release")
	err = f.LoadManifest()
	if assert.NoError(err) {
		f.Options.OutputFormat = "human"
		err = f.ListProperties()
		assert.NoError(err, "Expected ListProperties to list release properties for human consumption")

		f.Options.OutputFormat = "json"
		err = f.ListProperties()
		assert.NoError(err, "Expected ListProperties to list release properties in JSON")

		f.Options.OutputFormat = "yaml"
		err = f.ListProperties()
		assert.NoError(err, "Expected ListProperties to list release properties in YAML")
	}
}

var testSerializeInput struct {
	releases []*model.Release
	once     sync.Once
}

func initTestSerializeInput() {
	releases := []*model.Release{
		&model.Release{
			Name: "first release",
			Jobs: model.Jobs{
				&model.Job{
					Name:        "first job",
					Description: "a first job",
					Fingerprint: "job-one",
					Templates: []*model.JobTemplate{
						&model.JobTemplate{
							SourcePath:      "/dev/urandom",
							DestinationPath: "/dev/null",
							Content:         "hello",
						},
					},
				},
			},
			Packages: model.Packages{
				&model.Package{
					Name:        "base package",
					Fingerprint: "abc",
					Path:        "/some/path",
					SHA1:        "123",
					Version:     "one",
				},
			},
			CommitHash:         "zzz",
			DevBOSHCacheDir:    "/bosh/cache/1",
			Path:               "/some/path/1",
			UncommittedChanges: false,
			Version:            "ONE",
		},
		&model.Release{
			Name: "second release",
			Jobs: model.Jobs{
				&model.Job{
					Name:        "second job",
					Description: "a second job",
					Fingerprint: "job-two",
				},
			},
			Packages: model.Packages{
				&model.Package{
					Name:        "dependent package",
					Fingerprint: "def",
					Path:        "/another/path",
					SHA1:        "456",
					Version:     "two",
				},
			},
			CommitHash:         "qqq",
			DevBOSHCacheDir:    "/bosh/cache/2",
			Path:               "/some/path/2",
			UncommittedChanges: true,
			Version:            "TWO",
		},
	}

	// Set up package dependencies + reference cycles via release
	var lastPkg *model.Package
	for _, r := range releases {
		for _, j := range r.Jobs {
			j.Packages = r.Packages
			j.Release = r
			for _, template := range j.Templates {
				template.Job = j
			}
		}
		for _, pkg := range r.Packages {
			pkg.Release = r
			if lastPkg != nil {
				pkg.Dependencies = append(pkg.Dependencies, lastPkg)
			}
			lastPkg = pkg
		}
	}

	testSerializeInput.releases = releases
}

func TestSerializePackages(t *testing.T) {
	assert := assert.New(t)
	testSerializeInput.once.Do(initTestSerializeInput)
	f := &Fissile{
		Manifest: &model.RoleManifest{
			LoadedReleases: testSerializeInput.releases,
		}}
	result, err := f.SerializePackages()
	if !assert.NoError(err) {
		return
	}
	actual, err := json.Marshal(result)
	if !assert.NoError(err) {
		return
	}
	expected := `{
		"abc": {
			"name": "base package",
			"fingerprint": "abc",
			"path": "/some/path",
			"sha1": "123",
			"version": "one",
			"dependencies": [],
			"release": "first release"
		},
		"def": {
			"name": "dependent package",
			"fingerprint": "def",
			"path": "/another/path",
			"sha1": "456",
			"version": "two",
			"dependencies": ["abc"],
			"release": "second release"
		}
	}`
	assert.JSONEq(expected, string(actual))

	_, err = (&Fissile{}).SerializeReleases()
	assert.EqualError(err, "Releases not loaded")
}

func TestSerializeReleases(t *testing.T) {
	assert := assert.New(t)
	testSerializeInput.once.Do(initTestSerializeInput)
	f := &Fissile{
		Manifest: &model.RoleManifest{
			LoadedReleases: testSerializeInput.releases,
		}}
	result, err := f.SerializeReleases()
	if !assert.NoError(err) {
		return
	}
	actual, err := json.Marshal(result)
	if !assert.NoError(err) {
		return
	}
	expected := `{
		"first release": {
			"name": "first release",
			"packages": ["abc"],
			"commitHash": "zzz",
			"devBOSHCacheDir": "/bosh/cache/1",
			"jobs": ["job-one"],
			"license": {},
			"path": "/some/path/1",
			"uncommittedChanges": false,
			"version": "ONE"
		},
		"second release": {
			"name": "second release",
			"packages": ["def"],
			"commitHash": "qqq",
			"devBOSHCacheDir": "/bosh/cache/2",
			"jobs": ["job-two"],
			"license": {},
			"path": "/some/path/2",
			"uncommittedChanges": true,
			"version": "TWO"
		}
	}`
	assert.JSONEq(expected, string(actual))

	_, err = (&Fissile{}).SerializeReleases()
	assert.EqualError(err, "Releases not loaded")
}

func TestSerializeJobs(t *testing.T) {
	assert := assert.New(t)
	testSerializeInput.once.Do(initTestSerializeInput)
	f := &Fissile{
		Manifest: &model.RoleManifest{
			LoadedReleases: testSerializeInput.releases,
		}}
	result, err := f.SerializeJobs()
	if !assert.NoError(err) {
		return
	}
	actual, err := json.Marshal(result)
	if !assert.NoError(err) {
		return
	}
	expected := `{
		"job-one": {
			"name": "first job",
			"fingerprint": "job-one",
			"packages": ["abc"],
			"release": "first release",
			"description": "a first job",
			"path": "",
			"properties": [],
			"sha1": "",
			"templates": [{
				"sourcePath": "/dev/urandom",
				"destinationPath": "/dev/null",
				"job": "job-one",
				"content": "hello"
			}],
			"version": ""
		},
		"job-two": {
			"name": "second job",
			"fingerprint": "job-two",
			"packages": ["def"],
			"release": "second release",
			"description": "a second job",
			"path": "",
			"properties": [],
			"sha1": "",
			"templates": [],
			"version": ""
		}
	}`
	assert.JSONEq(expected, string(actual))

	_, err = (&Fissile{}).SerializeJobs()
	assert.EqualError(err, "Releases not loaded")
}

func TestGenerateAuth(t *testing.T) {
	workDir, err := os.Getwd()
	require.NoError(t, err)

	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	f := NewFissileApplication(".", ui)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/app/generate-auth.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../test-assets/.final_releases")},
		Grapher: f})
	require.NoError(t, err)
	require.NotNil(t, roleManifest)

	// Force the usage counters for accounts and roles to values
	// which causes generateAuth to write the proper files.
	for accountName, accountSpec := range roleManifest.Configuration.Authorization.Accounts {
		accountSpec.UsedBy = map[string]struct{}{
			"foo": struct{}{},
			"bar": struct{}{},
		}
		roleManifest.Configuration.Authorization.Accounts[accountName] = accountSpec
	}
	for roleName := range roleManifest.Configuration.Authorization.Roles {
		roleManifest.Configuration.Authorization.RoleUsedBy[roleName] = map[string]struct{}{
			"account-1": struct{}{},
			"account-2": struct{}{},
		}
	}

	outDir, err := ioutil.TempDir("", "fissile-generate-auth-")
	require.NoError(t, err)
	defer os.RemoveAll(outDir)

	settings := kube.ExportSettings{
		OutputDir:    outDir,
		RoleManifest: roleManifest,
	}
	err = f.generateAuth(settings)
	require.NoError(t, err)

	samples := map[string][]string{
		`auth/auth-role-extra-permissions.yaml`: []string{
			`{
				"apiVersion": "rbac.authorization.k8s.io/v1",
				"kind": "Role",
				"metadata": {
					"name": "extra-permissions"
				},
				"rules": [
					{
						"apiGroups": [""],
						"resources": ["pods"],
						"verbs": ["create", "get", "list", "update", "patch", "delete"]
					}
				]
			}`,
		},
		`auth/auth-role-pointless.yaml`: []string{
			`{
				"apiVersion": "rbac.authorization.k8s.io/v1",
				"kind": "Role",
				"metadata": {
					"name": "pointless"
				},
				"rules": [
					{
						"apiGroups": [""],
						"resources": ["bird"],
						"verbs": ["fly"]
					}
				]
			}`,
		},
		`auth/account-non-default.yaml`: []string{
			`{
				"apiVersion": "v1",
				"kind": "ServiceAccount",
				"metadata": {
					"name": "non-default"
				}
			}`,
			`{
				"apiVersion": "rbac.authorization.k8s.io/v1",
				"kind": "RoleBinding",
				"metadata": {
					"name": "non-default-extra-permissions-binding"
				},
				"subjects": [
					{
						"kind": "ServiceAccount",
						"name": "non-default"
					}
				],
				"roleRef": {
					"kind": "Role",
					"name": "extra-permissions",
					"apiGroup": "rbac.authorization.k8s.io"
				}
			}`,
			`{
				"apiVersion": "rbac.authorization.k8s.io/v1",
				"kind": "ClusterRole",
				"metadata": {
					"name": "nonprivileged"
				},
				"rules": [
					{
						"apiGroups": ["extensions"],
						"resourceNames": ["nonprivileged"],
						"resources": ["podsecuritypolicies"],
						"verbs": ["use"]
					}
				]
			}`,
			`{
				"apiVersion": "rbac.authorization.k8s.io/v1",
				"kind": "ClusterRoleBinding",
				"metadata": {
					"name": "non-default-nonprivileged-cluster-binding"
				},
				"subjects": [
					{
						"kind": "ServiceAccount",
						"name": "non-default",
						"namespace": "~"
					}
				],
				"roleRef": {
					"kind": "ClusterRole",
					"name": "nonprivileged",
					"apiGroup": "rbac.authorization.k8s.io"
				}
			}`,
		},
		`auth/account-default.yaml`: []string{
			// Service accounts named "default" should not get created
			`{
				"apiVersion": "rbac.authorization.k8s.io/v1",
				"kind": "RoleBinding",
				"metadata": {
					"name": "default-pointless-binding"
				},
				"subjects": [
					{
						"kind": "ServiceAccount",
						"name": "default"
					}
				],
				"roleRef": {
					"kind": "Role",
					"name": "pointless",
					"apiGroup": "rbac.authorization.k8s.io"
				}
			}`,
			// No cluster role binding is specified for this account
		},
		`auth/auth-psp-nonprivileged.yaml`: []string{
			`{
				"apiVersion": "policy/v1beta1",
				"kind": "PodSecurityPolicy",
				"metadata": {
					"name": "nonprivileged",
					"labels": {
						"app.kubernetes.io/component": "nonprivileged"
					}
				},
				"spec": {
					"runAsUser": {
						"rule": "RunAsAny"
					}
				}
			}`,
		},
	}

	var sampleNames []string
	for sampleName := range samples {
		sampleNames = append(sampleNames, sampleName)
	}
	err = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(outDir, path)
		if err != nil {
			return err
		}
		contents, err := ioutil.ReadFile(path)
		if assert.NoError(t, err, fmt.Sprintf("Error reading unexpected file %s", relPath)) {
			assert.Contains(t, sampleNames, relPath,
				"Unexpected file %s with contents: \n%s\n",
				relPath, contents)
		}
		return nil
	})
	assert.NoError(t, err, "Unexpected error looking for extra files")

	for name, expectedText := range samples {
		t.Run(name, func(t *testing.T) {
			actualText, err := ioutil.ReadFile(filepath.Join(outDir, name))
			require.NoError(t, err)
			actualText = []byte(strings.TrimPrefix(string(actualText), "---\n"))
			actualChunks := strings.Split(string(actualText), "---\n")

			assert.Len(t, actualChunks, len(expectedText), "Unexpected number of chunks")

			for i, expectedChunk := range expectedText {
				// Run _another_ subtest so that we know which resource failed
				t.Run("", func(t *testing.T) {
					if i >= len(actualChunks) {
						// Already caught with the Len() assertion above
						return
					}
					var expected, actual map[string]interface{}
					err = json.Unmarshal([]byte(expectedChunk), &expected)
					assert.NoError(t, err, "Failed to unmarshal expected data")

					err = yaml.Unmarshal([]byte(actualChunks[i]), &actual)
					assert.NoError(t, err, "Failed to unmarshal actual results")

					testhelpers.IsYAMLSubset(assert.New(t), expected, actual)
				})
			}
		})
	}
}

func TestDevDiffConfigurations(t *testing.T) {
	assert := assert.New(t)
	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePathV215 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-215")
	releasePathV224 := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cf-release-224")
	cachePath := filepath.Join(workDir, "../test-assets/test-dev-config-diff/cache")

	release215, err := model.NewDevRelease(releasePathV215, "", "", cachePath)
	if !assert.NoError(err) {
		return
	}
	assert.NotNil(release215)
	release224, err := model.NewDevRelease(releasePathV224, "", "", cachePath)
	assert.NoError(err)
	assert.NotNil(release224)

	assert.Len(release215.Packages, 11) // temp #
	assert.Len(release224.Packages, 9)

	hashDiffs, err := getDiffsFromReleases([]*model.Release{release215, release224})
	if !assert.Nil(err, "getDiffsFromReleases failed") {
		return
	}
	if assert.Len(hashDiffs.AddedKeys, 4, fmt.Sprintf("Expected 4 added key, got %d: %s", len(hashDiffs.AddedKeys), hashDiffs.AddedKeys)) {
		sort.Strings(hashDiffs.AddedKeys)
		assert.Equal("acceptance_tests.include_route_services", hashDiffs.AddedKeys[0])
		assert.Equal("app_ssh.oauth_client_id", hashDiffs.AddedKeys[1])
		assert.Equal("cf:acceptance-tests:acceptance_tests.include_route_services", hashDiffs.AddedKeys[2])
		assert.Equal("cf:cloud_controller_ng:app_ssh.oauth_client_id", hashDiffs.AddedKeys[3])
	}
	if assert.Len(hashDiffs.DeletedKeys, 4, fmt.Sprintf("Expected 4 dropped key, got %d: %s", len(hashDiffs.DeletedKeys), hashDiffs.DeletedKeys)) {
		sort.Strings(hashDiffs.DeletedKeys)
		assert.Equal("acceptance_tests.old_key", hashDiffs.DeletedKeys[0])
		assert.Equal("cf:acceptance-tests:acceptance_tests.old_key", hashDiffs.DeletedKeys[1])
		assert.Equal("cf:cloud_controller_ng:networks.apps", hashDiffs.DeletedKeys[2])
		assert.Equal("networks.apps", hashDiffs.DeletedKeys[3])
	}
	assert.Len(hashDiffs.ChangedValues, 5)
	v, ok := hashDiffs.ChangedValues["cc.staging_upload_user"]
	if assert.True(ok) {
		assert.Equal("S3 Access key for staging droplets on AWS installs; Blobstore user for other IaaSs", v[0])
		assert.Equal("User name used to access internal endpoints of Cloud Controller to upload files when staging", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf:cloud_controller_ng:cc.external_protocol"]
	if assert.True(ok) {
		assert.Equal("http", v[0])
		assert.Equal("https", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf:acceptance-tests:acceptance_tests.fake_key"]
	if assert.True(ok) {
		assert.Equal("49", v[0])
		assert.Equal("10", v[1])
	}
	v, ok = hashDiffs.ChangedValues["acceptance_tests.use_diego"]
	if assert.True(ok) {
		assert.Equal("Services tests push their apps using diego if enabled", v[0])
		assert.Equal("App tests push their apps using diego if enabled. Route service tests require this flag to run.", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf:cloud_controller_ng:metron_endpoint.port"]
	if assert.True(ok) {
		assert.Equal("3456", v[0])
		assert.Equal("3457", v[1])
	}
	v, ok = hashDiffs.ChangedValues["cf.bogus.key"]
	assert.False(ok)
}

func TestFissileSelectRolesToBuild(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	workDir, err := os.Getwd()
	require.NoError(t, err)

	// Set up the test params
	f := NewFissileApplication(",", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/roles-to-build.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	require.NoError(t, err)

	roleManifest := f.Manifest

	testSamples := []struct {
		roleNames     []string
		expectedNames []string
		err           string
	}{
		{
			roleNames:     []string{"myrole", "foorole"},
			expectedNames: []string{"foorole", "myrole"},
		},
		{
			roleNames:     []string{"myrole"},
			expectedNames: []string{"myrole"},
		},
		{
			roleNames: []string{"missing_role"},
			err:       "Some instance groups are unknown: [missing_role]",
		},
	}

	for _, sample := range testSamples {
		t.Run(strings.Join(sample.roleNames, ","), func(t *testing.T) {
			results, err := roleManifest.SelectInstanceGroups(sample.roleNames)
			if sample.err != "" {
				assert.EqualError(t, err, sample.err, "while testing %v", sample.roleNames)
			} else {
				assert.NoError(t, err, "while testing %v", sample.roleNames)
				var actualNames []string
				for _, instanceGroup := range results {
					actualNames = append(actualNames, instanceGroup.Name)
				}
				sort.Strings(actualNames)
				assert.Equal(t, sample.expectedNames, actualNames, "while testing %v", sample.roleNames)
			}
		})
	}
}

func TestFissileGetReleasesByName(t *testing.T) {
	assert := assert.New(t)
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	workDir, err := os.Getwd()
	assert.NoError(err)

	f := NewFissileApplication(",", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/no-instance-groups/no-instance-groups.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/extracted-license"))
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/extracted-license"))
	f.Options.ReleaseNames = append(f.Options.ReleaseNames, "test-dev", "test2")
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	require.NoError(t, err, "failed to load role manifest")

	releases, err := f.getReleasesByName([]string{"test-dev"})
	if assert.NoError(err) {
		if assert.Len(releases, 1, "should have exactly one matching release") {
			assert.Equal("test-dev", releases[0].Name, "release has unexpected name")
		}
	}
	releases, err = f.getReleasesByName([]string{"test2", "test-dev"})
	if assert.NoError(err) {
		if assert.Len(releases, 2, "should have exactly two matching releases") {
			assert.Equal("test2", releases[0].Name, "first release has unexpected name")
			assert.Equal("test-dev", releases[1].Name, "second release has unexpected name")
		}
	}
	releases, err = f.getReleasesByName([]string{})
	if assert.NoError(err) {
		if assert.Len(releases, 2, "not specifying releases should return all releases") {
			assert.Equal("test-dev", releases[0].Name, "first release has unexpected name")
			assert.Equal("test2", releases[1].Name, "second release has unexpected name")
		}
	}
	_, err = f.getReleasesByName([]string{"test-dev", "missing"})
	if assert.Error(err, "Getting a non-existant release should result in an error") {
		assert.Contains(err.Error(), "missing", "Error message should contain missing release name")
		assert.NotContains(err.Error(), "test-dev", "Error message should not contain valid release name")
	}
}

func TestFissileGenerateKubeRoles(t *testing.T) {
	ui := termui.New(&bytes.Buffer{}, ioutil.Discard, nil)
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	// Set up the test params
	f := NewFissileApplication(".", ui)
	f.Options.RoleManifest = filepath.Join(workDir, "../test-assets/role-manifests/app/two-roles.yml")
	f.Options.Releases = append(f.Options.Releases, filepath.Join(workDir, "../test-assets/tor-boshrelease"))
	f.Options.CacheDir = filepath.Join(workDir, "../test-assets/bosh-cache")

	err = f.LoadManifest()
	require.NoError(t, err, "Failed to load release from %s", f.Options.Releases[0])

	roleManifest := f.Manifest

	outDir, err := ioutil.TempDir("", "fissile-test-generate-kube-roles")
	require.NoError(t, err)
	defer os.RemoveAll(outDir)

	err = f.generateKubeRoles(kube.ExportSettings{OutputDir: outDir, RoleManifest: roleManifest})
	assert.NoError(t, err)

	for _, name := range []string{"myrole-deployment.yaml", "myrole-clustered.yaml"} {
		path := filepath.Join(outDir, "bosh", name)
		_, err := os.Stat(path)
		assert.NoError(t, err, "Failed to find output %s", name)
	}
}
