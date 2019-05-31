package resolver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/loader"
	"code.cloudfoundry.org/fissile/model/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRoleManifestOK(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/tor-good.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			ReleaseNames:     []string{},
			ReleaseVersions:  []string{},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases"),
		},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	assert.Equal(t, roleManifestPath, roleManifest.ManifestFilePath)
	assert.Len(t, roleManifest.InstanceGroups, 2)

	myrole := roleManifest.InstanceGroups[0]
	assert.Equal(t, []string{
		"scripts/myrole.sh",
		"/script/with/absolute/path.sh",
	}, myrole.Scripts)

	foorole := roleManifest.InstanceGroups[1]
	torjob := foorole.JobReferences[0]
	assert.Equal(t, "tor", torjob.Name)
	assert.NotNil(t, torjob.Release)
	assert.Equal(t, "tor", torjob.Release.Name)
}

func TestScriptPathInvalid(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/script-bad-prefix.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})
	require.Error(t, err, "invalid role manifest should return error")
	assert.Nil(t, roleManifest, "invalid role manifest loaded")
	for _, msg := range []string{
		`myrole environment script: Invalid value: "lacking-prefix.sh": Script path does not start with scripts/`,
		`myrole script: Invalid value: "scripts/missing.sh": script not found`,
		`myrole post config script: Invalid value: "": script not found`,
	} {
		assert.Contains(t, err.Error(), msg, "missing expected validation error")
	}
	for _, msg := range []string{
		`myrole environment script: Invalid value: "scripts/environ.sh":`,
		`myrole environment script: Invalid value: "/environ/script/with/absolute/path.sh":`,
		`myrole script: Invalid value: "scripts/myrole.sh":`,
		`myrole script: Invalid value: "/script/with/absolute/path.sh":`,
		`myrole post config script: Invalid value: "scripts/post_config_script.sh":`,
		`myrole post config script: Invalid value: "/var/vcap/jobs/myrole/pre-start":`,
		`myrole post config script: Invalid value: "scripts/nested/run.sh":`,
		`scripts/nested: Required value: Script is not used`,
	} {
		assert.NotContains(t, err.Error(), msg, "unexpected validation error")
	}
}

func TestLoadRoleManifestNotOKBadJobName(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/tor-bad.yml")
	_, err = loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Cannot find job foo in release")
	}
}

func TestLoadDuplicateReleases(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/tor-good.yml")
	_, err = loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "release tor has been loaded more than once")
	}
}

func TestLoadRoleManifestMultipleReleasesOK(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/multiple-good.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	assert.Equal(t, roleManifestPath, roleManifest.ManifestFilePath)
	assert.Len(t, roleManifest.InstanceGroups, 2)

	myrole := roleManifest.InstanceGroups[0]
	assert.Len(t, myrole.Scripts, 1)
	assert.Equal(t, "scripts/myrole.sh", myrole.Scripts[0])

	foorole := roleManifest.InstanceGroups[1]
	torjob := foorole.JobReferences[0]
	assert.Equal(t, "tor", torjob.Name)
	if assert.NotNil(t, torjob.Release) {
		assert.Equal(t, "tor", torjob.Release.Name)
	}
}

func TestLoadRoleManifestMultipleReleasesNotOk(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/multiple-bad.yml")
	_, err = loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(),
			`instance_groups[foorole].jobs[ntpd]: Invalid value: "foo": Referenced release is not loaded`)
	}
}

func TestNonBoshRolesAreNotAllowed(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/non-bosh-roles.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})
	assert.EqualError(t, err, "instance_groups[dockerrole].type: Invalid value: \"docker\": Expected one of bosh, bosh-task, or colocated-container")
	assert.Nil(t, roleManifest)
}

func TestLoadRoleManifestVariablesPreviousNamesError(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/variables-with-dup-prev-names.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})
	require.Error(t, err)

	assert.Contains(t, err.Error(), `variables: Invalid value: "FOO": Previous name 'BAR' also exist as a new variable`)
	assert.Contains(t, err.Error(), `variables: Invalid value: "FOO": Previous name 'BAZ' also claimed by 'QUX'`)
	assert.Contains(t, err.Error(), `variables: Invalid value: "QUX": Previous name 'BAZ' also claimed by 'FOO'`)
	// Note how this ignores other errors possibly present in the manifest and releases.
	assert.Nil(t, roleManifest)
}

func TestLoadRoleManifestVariablesSSH(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/variables-ssh.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})

	assert.NoError(t, err)
	assert.NotNil(t, roleManifest)
}

func TestLoadRoleManifestBadType(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/bad-type.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})

	require.Contains(t, err.Error(),
		`variables[BAR].type: Invalid value: "invalid": Expected one of certificate, password, rsa, ssh or empty`)
	require.Contains(t, err.Error(),
		`variables[FOO].type: Invalid value: "rsa": The rsa type is not yet supported by the secret generator`)
	assert.Nil(t, roleManifest)
}

func TestLoadRoleManifestBadCVType(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/bad-cv-type.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})

	require.EqualError(t, err,
		`variables[BAR].options.type: Invalid value: "bogus": Expected one of user, or environment`)
	assert.Nil(t, roleManifest)
}

func TestLoadRoleManifestBadCVTypeConflictInternal(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/bad-cv-type-internal.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.EqualError(t, err,
		`variables[BAR].options.type: Invalid value: "environment": type conflicts with flag "internal"`)
	assert.Nil(t, roleManifest)
}

func TestLoadRoleManifestMissingRBACAccount(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/rbac-missing-account.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})
	assert.EqualError(t, err, `instance_groups[myrole].run.service-account: Not found: "missing-account"`)
	assert.Nil(t, roleManifest)
}

func TestLoadRoleManifestTemplatedRBACAccount(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/rbac-templated-account.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(t, err)
	assert.NotNil(t, roleManifest)
}

func TestLoadRoleManifestMissingRBACRole(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/rbac-missing-role.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.EqualError(t, err, `configuration.auth.accounts[test-account].roles: Not found: "missing-role"`)
	assert.Nil(t, roleManifest)
}

func TestLoadRoleManifestRunGeneral(t *testing.T) {
	t.Parallel()

	workDir, err := os.Getwd()
	require.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")

	type testCase struct {
		manifest string
		message  []string
	}

	tests := []testCase{
		{
			"bosh-run-missing.yml", []string{
				"instance_groups[myrole]: Required value: `properties.bosh_containerization.run` required for at least one Job",
			},
		},
		{
			"bosh-run-bad-proto.yml", []string{
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[https].protocol: Unsupported value: "AA": supported values: TCP, UDP`,
			},
		},
		{
			"bosh-run-bad-port-names.yml", []string{
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[a--b].name: Invalid value: "a--b": port names must be lowercase words separated by hyphens`,
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[abcd-efgh-ijkl-x].name: Invalid value: "abcd-efgh-ijkl-x": port name must be no more than 15 characters`,
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[abcdefghij].name: Invalid value: "abcdefghij": user configurable port name must be no more than 9 characters`,
			},
		},
		{
			"bosh-run-bad-port-count.yml", []string{
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[http].count: Invalid value: 2: count doesn't match port range 80-82`,
			},
		},
		{
			"bosh-run-bad-ports.yml", []string{
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[https].internal: Invalid value: "-1": invalid syntax`,
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[https].external: Invalid value: 0: must be between 1 and 65535, inclusive`,
			},
		},
		{
			"bosh-run-missing-portrange.yml", []string{
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[https].internal: Invalid value: "": invalid syntax`,
			},
		},
		{
			"bosh-run-reverse-portrange.yml", []string{
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[https].internal: Invalid value: "5678-123": last port can't be lower than first port`,
			},
		},
		{
			"bosh-run-bad-parse.yml", []string{
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[https].internal: Invalid value: "qq": invalid syntax`,
				`instance_groups[myrole].jobs[tor].properties.bosh_containerization.ports[https].external: Invalid value: "aa": invalid syntax`,
			},
		},
		{
			"bosh-run-bad-memory.yml", []string{
				`instance_groups[myrole].run.memory: Invalid value: -10: must be greater than or equal to 0`,
			},
		},
		{
			"bosh-run-bad-cpu.yml", []string{
				`instance_groups[myrole].run.virtual-cpus: Invalid value: -2: must be greater than or equal to 0`,
			},
		},
		{
			"bosh-run-ok.yml", []string{},
		},
	}

	for _, tc := range tests {
		func(tc testCase) {
			t.Run(tc.manifest, func(t *testing.T) {
				t.Parallel()
				roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model", tc.manifest)
				roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
					ReleaseOptions: model.ReleaseOptions{
						ReleasePaths:     []string{torReleasePath},
						BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
						FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
					ValidationOptions: model.RoleManifestValidationOptions{
						AllowMissingScripts: true,
					}})

				if len(tc.message) > 0 {
					assert.EqualError(t, err, strings.Join(tc.message, "\n"))
					assert.Nil(t, roleManifest)
				} else {
					assert.NoError(t, err)
				}
			})
		}(tc)
	}
}

func TestResolveLinks(t *testing.T) {
	workDir, err := os.Getwd()

	assert.NoError(t, err)

	releasePaths := []string{}

	for _, dirName := range []string{"ntp-release", "tor-boshrelease"} {
		releasePath := filepath.Join(workDir, "../../test-assets", dirName)
		releasePaths = append(releasePaths, releasePath)
	}

	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/multiple-good.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     releasePaths,
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	// loader.LoadRoleManifest implicitly runs resolveLinks()
	role := roleManifest.LookupInstanceGroup("myrole")
	job := role.LookupJob("ntpd")
	require.NotNil(t, job)

	t.Run("consumes", func(t *testing.T) {
		// Comparing things with assert.Equal() just gives us impossible-to-read dumps
		samples := []struct {
			Name     string
			Type     string
			Optional bool
			Missing  bool
		}{
			// These should match the order in the ntp-release ntp job.MF
			{Name: "ntp-server", Type: "ntpd"},
			{Name: "ntp-client", Type: "ntp"},
			{Type: "missing", Missing: true},
		}

		expectedLength := 0
		for _, expected := range samples {
			t.Run("", func(t *testing.T) {
				if expected.Missing {
					for name, consumeInfo := range job.ResolvedConsumes {
						assert.NotEqual(t, expected.Type, consumeInfo.Type,
							"link should not resolve, got %s (type %s) in %s / %s",
							name, consumeInfo.Type, consumeInfo.RoleName, consumeInfo.JobName)
					}
					return
				}
				expectedLength++
				require.Contains(t, job.ResolvedConsumes, expected.Name, "link %s is missing", expected.Name)
				actual := job.ResolvedConsumes[expected.Name]
				assert.Equal(t, expected.Name, actual.Name, "link name mismatch")
				assert.Equal(t, expected.Type, actual.Type, "link type mismatch")
				assert.Equal(t, role.Name, actual.RoleName, "link role name mismatch")
				assert.Equal(t, job.Name, actual.JobName, "link job name mismatch")
			})
		}
		assert.Len(t, job.ResolvedConsumes, expectedLength)
	})

	t.Run("consumed-by", func(t *testing.T) {
		expected := map[string][]model.JobLinkInfo{
			"ntp-client": []model.JobLinkInfo{
				{RoleName: "myrole", JobName: "tor"},
				{RoleName: "myrole", JobName: "ntpd"},
				{RoleName: "foorole", JobName: "tor"},
			},
			"ntp-server": []model.JobLinkInfo{
				{RoleName: "myrole", JobName: "ntpd"},
			},
		}
		for linkName, expectedConsumedByList := range expected {
			t.Run(linkName, func(t *testing.T) {
				consumedByList, ok := job.ResolvedConsumedBy[linkName]
				require.True(t, ok, "Could not find consumed-by for link %s", linkName)
				for _, expectedConsumedBy := range expectedConsumedByList {
					found := false
					for _, consumedBy := range consumedByList {
						if consumedBy.RoleName != expectedConsumedBy.RoleName {
							continue
						}
						if consumedBy.JobName != expectedConsumedBy.JobName {
							continue
						}
						assert.False(t, found,
							"Found duplicate consumed-by info for link %s with instance group %s job %s",
							linkName, expectedConsumedBy.RoleName, expectedConsumedBy.JobName)
						found = true
					}
					assert.True(t, found,
						"Could not find consumed-by for link %s with instance group %s job %s",
						linkName, expectedConsumedBy.RoleName, expectedConsumedBy.JobName)
				}
			})
		}
	})
}

func TestRoleResolveLinksMultipleProvider(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	job1 := &model.Job{
		Name: "job-1",
		AvailableProviders: map[string]model.JobProvidesInfo{
			"job-1-provider-1": {
				JobLinkInfo: model.JobLinkInfo{
					Name: "job-1-provider-1",
					Type: "link-1",
				},
			},
			"job-1-provider-2": {
				JobLinkInfo: model.JobLinkInfo{
					Name: "job-1-provider-2",
					Type: "link-2",
				},
			},
			"job-1-provider-3": {
				JobLinkInfo: model.JobLinkInfo{
					Name: "job-1-provider-3",
					Type: "link-5",
				},
			},
		},
		DesiredConsumers: []model.JobConsumesInfo{
			{
				JobLinkInfo: model.JobLinkInfo{
					Name: "job-1-provider-1",
					Type: "link-1",
				},
			},
		},
	}

	job2 := &model.Job{
		Name: "job-2",
		AvailableProviders: map[string]model.JobProvidesInfo{
			"job-2-provider-1": {
				JobLinkInfo: model.JobLinkInfo{
					Name: "job-2-provider-1",
					Type: "link-3",
				},
			},
		},
	}

	job3 := &model.Job{
		Name: "job-3",
		AvailableProviders: map[string]model.JobProvidesInfo{
			"job-3-provider-3": {
				JobLinkInfo: model.JobLinkInfo{
					Name: "job-3-provider-3",
					Type: "link-4",
				},
			},
		},
		DesiredConsumers: []model.JobConsumesInfo{
			{
				// There is exactly one implicit provider of this type; use it
				JobLinkInfo: model.JobLinkInfo{
					Type: "link-1", // j1
				},
			},
			{
				// This job has multiple available implicit providers with
				// the same type; this should not resolve.
				JobLinkInfo: model.JobLinkInfo{
					Type: "link-3", // j3
				},
				Optional: true,
			},
			{
				// There is exactly one explicit provider of this name
				JobLinkInfo: model.JobLinkInfo{
					Name: "job-3-provider-3", // j3
				},
			},
			{
				// There are no providers of this type
				JobLinkInfo: model.JobLinkInfo{
					Type: "missing",
				},
				Optional: true,
			},
			{
				// This requires an alias
				JobLinkInfo: model.JobLinkInfo{
					Name: "actual-consumer-name",
				},
				Optional: true, // Not resolvable in role 3
			},
		},
	}

	roleManifest := &model.RoleManifest{
		InstanceGroups: model.InstanceGroups{
			&model.InstanceGroup{
				Name: "role-1",
				JobReferences: model.JobReferences{
					{
						Job: job1,
						ExportedProvides: map[string]model.JobProvidesInfo{
							"job-1-provider-3": model.JobProvidesInfo{
								Alias: "unique-alias",
							},
						},
						ContainerProperties: model.JobContainerProperties{
							BoshContainerization: model.JobBoshContainerization{
								ServiceName: "job-1-service",
							},
						},
					},
					{Job: job2},
				},
			},
			&model.InstanceGroup{
				Name: "role-2",
				JobReferences: model.JobReferences{
					{Job: job2},
					{
						Job: job3,
						// This has an explicitly exported provider
						ExportedProvides: map[string]model.JobProvidesInfo{
							"job-3-provider-3": model.JobProvidesInfo{},
						},
						ResolvedConsumes: map[string]model.JobConsumesInfo{
							"actual-consumer-name": model.JobConsumesInfo{
								Alias: "unique-alias",
							},
						},
					},
				},
			},
			&model.InstanceGroup{
				Name: "role-3",
				// This does _not_ have an explicitly exported provider
				JobReferences: model.JobReferences{{Job: job2}, {Job: job3}},
			},
		},
	}
	for _, r := range roleManifest.InstanceGroups {
		for _, jobReference := range r.JobReferences {
			jobReference.Name = jobReference.Job.Name
			if jobReference.ResolvedConsumes == nil {
				jobReference.ResolvedConsumes = make(map[string]model.JobConsumesInfo)
			}
			if jobReference.ResolvedConsumedBy == nil {
				jobReference.ResolvedConsumedBy = make(map[string][]model.JobLinkInfo)
			}
		}
	}
	errors := resolver.NewResolver(roleManifest, nil, model.LoadRoleManifestOptions{}).ResolveLinks()
	assert.Empty(errors)
	role := roleManifest.LookupInstanceGroup("role-2")
	require.NotNil(role, "Failed to find role")
	job := role.LookupJob("job-3")
	require.NotNil(job, "Failed to find job")
	consumes := job.ResolvedConsumes

	assert.Len(consumes, 3, "incorrect number of resulting link consumers")

	if assert.Contains(consumes, "job-1-provider-1", "failed to find role by type") {
		assert.Equal(model.JobConsumesInfo{
			JobLinkInfo: model.JobLinkInfo{
				Name:        "job-1-provider-1",
				Type:        "link-1",
				RoleName:    "role-1",
				JobName:     "job-1",
				ServiceName: "job-1-service",
			},
		}, consumes["job-1-provider-1"], "found incorrect role by type")
	}

	assert.NotContains(consumes, "job-3-provider-1",
		"should not automatically resolve consumers with multiple providers of the type")

	if assert.Contains(consumes, "job-3-provider-3", "did not find explicitly named provider") {
		assert.Equal(model.JobConsumesInfo{
			JobLinkInfo: model.JobLinkInfo{
				Name:        "job-3-provider-3",
				Type:        "link-4",
				RoleName:    "role-2",
				JobName:     "job-3",
				ServiceName: "role-2-job-3",
			},
		}, consumes["job-3-provider-3"], "did not find explicitly named provider")
	}

	if assert.Contains(consumes, "actual-consumer-name", "did not resolve consumer with alias") {
		assert.Equal(model.JobConsumesInfo{
			JobLinkInfo: model.JobLinkInfo{
				Name:        "job-1-provider-3",
				Type:        "link-5",
				RoleName:    "role-1",
				JobName:     "job-1",
				ServiceName: "job-1-service",
			},
		}, consumes["actual-consumer-name"], "resolved to incorrect provider for alias")
	}
}

func TestLoadRoleManifestColocatedContainers(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/colocated-containers.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(err)
	assert.NotNil(roleManifest)

	assert.Len(roleManifest.InstanceGroups, 2)
	assert.EqualValues(model.RoleTypeBosh, roleManifest.LookupInstanceGroup("main-role").Type)
	assert.EqualValues(model.RoleTypeColocatedContainer, roleManifest.LookupInstanceGroup("to-be-colocated").Type)
	assert.Len(roleManifest.LookupInstanceGroup("main-role").ColocatedContainers(), 1)

	for _, roleName := range []string{"main-role", "to-be-colocated"} {
		assert.EqualValues([]*model.RoleRunVolume{&model.RoleRunVolume{Path: "/var/vcap/store", Type: "emptyDir", Tag: "shared-data"}}, roleManifest.LookupInstanceGroup(roleName).Run.Volumes)
	}
}

func TestLoadRoleManifestColocatedContainersValidationMissingRole(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/colocated-containers-with-missing-role.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")}})
	assert.Nil(roleManifest)
	assert.EqualError(err, `instance_groups[main-role].colocated_containers[0]: Invalid value: "to-be-colocated-typo": There is no such instance group defined`)
}

func TestLoadRoleManifestColocatedContainersValidationUsusedRole(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/colocated-containers-with-unused-role.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.Nil(roleManifest)
	assert.EqualError(err, "instance_group[to-be-colocated].job[ntpd].consumes[ntp-server]: Required value: failed to resolve provider ntp-server (type ntpd)\n"+
		"instance_group[orphaned].job[ntpd].consumes[ntp-server]: Required value: failed to resolve provider ntp-server (type ntpd)\n"+
		"instance_group[orphaned]: Not found: \"instance group is of type colocated container, but is not used by any other instance group as such\"")
}

func TestLoadRoleManifestColocatedContainersValidationPortCollisions(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/colocated-containers-with-port-collision.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.Nil(roleManifest)
	assert.EqualError(err, "instance_group[main-role]: Invalid value: \"TCP/10443\": port collision, the same protocol/port is used by: main-role, to-be-colocated"+"\n"+
		"instance_group[main-role]: Invalid value: \"TCP/80\": port collision, the same protocol/port is used by: main-role, to-be-colocated")
}

func TestLoadRoleManifestColocatedContainersValidationPortCollisionsWithProtocols(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/colocated-containers-with-no-port-collision.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(err)
	assert.NotNil(roleManifest)
}

func TestLoadRoleManifestColocatedContainersValidationInvalidTags(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")

	_, err = model.NewDevRelease(torReleasePath, "", "", filepath.Join(workDir, "../../test-assets/bosh-cache"))
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	_, err = model.NewDevRelease(ntpReleasePath, "", "", filepath.Join(workDir, "../../test-assets/bosh-cache"))
	assert.NoError(err)
}

func TestLoadRoleManifestColocatedContainersValidationOfSharedVolumes(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/colocated-containers-with-volume-share-issues.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.Nil(roleManifest)
	assert.EqualError(err, "instance_group[to-be-colocated]: Invalid value: \"/mnt/foobAr\": colocated instance group specifies a shared volume with tag mount-share, which path does not match the path of the main instance group shared volume with the same tag\n"+
		"instance_group[main-role]: Required value: container must use shared volumes of the main instance group: vcap-logs\n"+
		"instance_group[main-role]: Required value: container must use shared volumes of the main instance group: vcap-store")
}

func TestLoadRoleManifestColocatedContainersValidationOfMultipleColocatedContainersWithDifferentMounts(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	ntpReleasePath := filepath.Join(workDir, "../../test-assets/ntp-release")
	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/colocated-containers-with-different-volume-shares.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			ReleasePaths:     []string{torReleasePath, ntpReleasePath},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})

	assert.NotNil(roleManifest)
	assert.Nil(err)
}

func TestLoadRoleManifestWithReleaseReferences(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/online-release-references.yml")
	roleManifest, err := loader.LoadRoleManifest(roleManifestPath, model.LoadRoleManifestOptions{
		ReleaseOptions: model.ReleaseOptions{
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases")},
		ValidationOptions: model.RoleManifestValidationOptions{
			AllowMissingScripts: true,
		}})
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	assert.Equal(t, roleManifestPath, roleManifest.ManifestFilePath)
	assert.Len(t, roleManifest.InstanceGroups, 1)
}
