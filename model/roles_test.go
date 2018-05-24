package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRoleManifestOK(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.NoError(err)
	assert.NotNil(roleManifest)

	assert.Equal(roleManifestPath, roleManifest.manifestFilePath)
	assert.Len(roleManifest.Roles, 2)

	myrole := roleManifest.Roles[0]
	assert.Equal([]string{
		"myrole.sh",
		"/script/with/absolute/path.sh",
	}, myrole.Scripts)

	foorole := roleManifest.Roles[1]
	torjob := foorole.RoleJobs[0]
	assert.Equal("tor", torjob.Name)
	assert.NotNil(torjob.Release)
	assert.Equal("tor", torjob.Release.Name)
}

func TestGetScriptPaths(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.NoError(err)
	assert.NotNil(roleManifest)

	fullScripts := roleManifest.Roles[0].GetScriptPaths()
	assert.Len(fullScripts, 3)
	for _, leafName := range []string{"environ.sh", "myrole.sh", "post_config_script.sh"} {
		assert.Equal(filepath.Join(workDir, "../test-assets/role-manifests", leafName), fullScripts[leafName])
	}
}

func TestLoadRoleManifestNotOKBadJobName(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-bad.yml")
	_, err = LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.NotNil(err)
	assert.Contains(err.Error(), "Cannot find job foo in release")
}

func TestLoadDuplicateReleases(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	_, err = LoadRoleManifest(roleManifestPath, []*Release{release, release}, nil)

	assert.NotNil(err)
	assert.Contains(err.Error(), "release tor has been loaded more than once")
}

func TestLoadRoleManifestMultipleReleasesOK(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	torRelease, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/multiple-good.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{ntpRelease, torRelease}, nil)
	assert.NoError(err)
	require.NotNil(t, roleManifest)

	assert.Equal(roleManifestPath, roleManifest.manifestFilePath)
	assert.Len(roleManifest.Roles, 2)

	myrole := roleManifest.Roles[0]
	assert.Len(myrole.Scripts, 1)
	assert.Equal("myrole.sh", myrole.Scripts[0])

	foorole := roleManifest.Roles[1]
	torjob := foorole.RoleJobs[0]
	assert.Equal("tor", torjob.Name)
	assert.NotNil(torjob.Release)
	assert.Equal("tor", torjob.Release.Name)
}

func TestLoadRoleManifestMultipleReleasesNotOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	torRelease, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/multiple-bad.yml")
	_, err = LoadRoleManifest(roleManifestPath, []*Release{ntpRelease, torRelease}, nil)

	assert.NotNil(err)
	assert.Contains(err.Error(),
		`roles[foorole].jobs[ntpd]: Invalid value: "foo": Referenced release is not loaded`)
}

func TestNonBoshRolesAreIgnoredOK(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/non-bosh-roles.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.NoError(err)
	require.NotNil(t, roleManifest)

	assert.Equal(roleManifestPath, roleManifest.manifestFilePath)
	assert.Len(roleManifest.Roles, 2)
}

func TestRolesSort(t *testing.T) {
	assert := assert.New(t)

	roles := Roles{
		{Name: "aaa"},
		{Name: "bbb"},
	}
	sort.Sort(roles)
	assert.Equal(roles[0].Name, "aaa")
	assert.Equal(roles[1].Name, "bbb")

	roles = Roles{
		{Name: "ddd"},
		{Name: "ccc"},
	}
	sort.Sort(roles)
	assert.Equal(roles[0].Name, "ccc")
	assert.Equal(roles[1].Name, "ddd")
}

func TestGetScriptSignatures(t *testing.T) {
	assert := assert.New(t)

	refRole := &Role{
		Name: "bbb",
		RoleJobs: []*RoleJob{
			{
				Job: &Job{
					SHA1: "Role 2 Job 1",
					Packages: Packages{
						{Name: "aaa", SHA1: "Role 2 Job 1 Package 1"},
						{Name: "bbb", SHA1: "Role 2 Job 1 Package 2"},
					},
				},
			},
			{
				Job: &Job{
					SHA1: "Role 2 Job 2",
					Packages: Packages{
						{Name: "ccc", SHA1: "Role 2 Job 2 Package 1"},
					},
				},
			},
		},
	}

	firstHash, _ := refRole.GetScriptSignatures()

	workDir, err := ioutil.TempDir("", "fissile-test-")
	assert.NoError(err)
	defer os.RemoveAll(workDir)
	releasePath := filepath.Join(workDir, "role.yml")

	scriptName := "script.sh"
	scriptPath := filepath.Join(workDir, scriptName)
	err = ioutil.WriteFile(scriptPath, []byte("true\n"), 0644)
	assert.NoError(err)

	differentPatch := &Role{
		Name:     refRole.Name,
		RoleJobs: []*RoleJob{refRole.RoleJobs[0], refRole.RoleJobs[1]},
		Scripts:  []string{scriptName},
		roleManifest: &RoleManifest{
			manifestFilePath: releasePath,
		},
	}

	differentPatchHash, _ := differentPatch.GetScriptSignatures()
	assert.NotEqual(firstHash, differentPatchHash, "role hash should be dependent on patch string")

	err = ioutil.WriteFile(scriptPath, []byte("false\n"), 0644)
	assert.NoError(err)

	differentPatchFileHash, _ := differentPatch.GetScriptSignatures()
	assert.NotEqual(differentPatchFileHash, differentPatchHash, "role manifest hash should be dependent on patch contents")
}

func TestGetTemplateSignatures(t *testing.T) {
	assert := assert.New(t)

	differentTemplate1 := &Role{
		Name:     "aaa",
		RoleJobs: []*RoleJob{},
		Configuration: &Configuration{
			Templates: map[string]string{"foo": "bar"},
		},
	}

	differentTemplate2 := &Role{
		Name:     "aaa",
		RoleJobs: []*RoleJob{},
		Configuration: &Configuration{
			Templates: map[string]string{"bat": "baz"},
		},
	}

	differentTemplateHash1, _ := differentTemplate1.GetTemplateSignatures()
	differentTemplateHash2, _ := differentTemplate2.GetTemplateSignatures()
	assert.NotEqual(differentTemplateHash1, differentTemplateHash2, "template hash should be dependent on template contents")
}

func TestLoadRoleManifestVariablesSortedError(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/variables-badly-sorted.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)

	assert.Contains(err.Error(), `configuration.variables: Invalid value: "FOO": Does not sort before 'BAR'`)
	assert.Contains(err.Error(), `configuration.variables: Invalid value: "PELERINUL": Does not sort before 'ALPHA'`)
	assert.Contains(err.Error(), `configuration.variables: Invalid value: "PELERINUL": Appears more than once`)
	// Note how this ignores other errors possibly present in the manifest and releases.
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestVariablesPreviousNamesError(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/variables-with-dup-prev-names.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)

	assert.Contains(err.Error(), `configuration.variables: Invalid value: "FOO": Previous name 'BAR' also exist as a new variable`)
	assert.Contains(err.Error(), `configuration.variables: Invalid value: "FOO": Previous name 'BAZ' also claimed by 'QUX'`)
	assert.Contains(err.Error(), `configuration.variables: Invalid value: "QUX": Previous name 'BAZ' also claimed by 'FOO'`)
	// Note how this ignores other errors possibly present in the manifest and releases.
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestVariablesNotUsed(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/variables-without-usage.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.Equal(err.Error(),
		`configuration.variables: Not found: "No templates using 'SOME_VAR'"`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestVariablesNotDeclared(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/variables-without-decl.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.Equal(err.Error(),
		`configuration.variables: Not found: "No declaration of 'HOME'"`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestNonTemplates(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/templates-non.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.Equal(err.Error(),
		`configuration.templates: Invalid value: "": Using 'properties.tor.hostname' as a constant`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestBadCVType(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/bad-cv-type.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.Equal(err.Error(),
		`configuration.variables[BAR].type: Invalid value: "bogus": Expected one of user, or environment`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestBadCVTypeConflictInternal(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/bad-cv-type-internal.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.Equal(err.Error(),
		`configuration.variables[BAR].type: Invalid value: "environment": type conflicts with flag "internal"`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestRunEnvDocker(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/docker-run-env.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.EqualError(err, `roles[dockerrole].run.env: Not found: "No variable declaration of 'UNKNOWN'"`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestMissingRBACAccount(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/rbac-missing-account.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.EqualError(err, `roles[myrole].run.service-account: Not found: "missing-account"`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestMissingRBACRole(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/rbac-missing-role.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
	assert.EqualError(err, `configuration.auth.accounts[test-account].roles: Not found: "missing-role"`)
	assert.Nil(roleManifest)
}

func TestLoadRoleManifestRunGeneral(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torReleasePathBoshCache := filepath.Join(torReleasePath, "bosh-cache")
	release, err := NewDevRelease(torReleasePath, "", "", torReleasePathBoshCache)
	assert.NoError(err)

	tests := []struct {
		manifest string
		message  []string
	}{
		{
			"bosh-run-missing.yml", []string{
				`roles[myrole].run: Required value`,
			},
		},
		{
			"bosh-run-bad-proto.yml", []string{
				`roles[myrole].run.exposed-ports[https].protocol: Unsupported value: "AA": supported values: TCP, UDP`,
			},
		},
		{
			"bosh-run-bad-port-names.yml", []string{
				`roles[myrole].run.exposed-ports[a--b].name: Invalid value: "a--b": port names must be lowercase words separated by hyphens`,
				`roles[myrole].run.exposed-ports[abcd-efgh-ijkl-x].name: Invalid value: "abcd-efgh-ijkl-x": port name must be no more than 15 characters`,
				`roles[myrole].run.exposed-ports[abcdefghij].name: Invalid value: "abcdefghij": user configurable port name must be no more than 9 characters`,
			},
		},
		{
			"bosh-run-bad-port-count.yml", []string{
				`roles[myrole].run.exposed-ports[http].count: Invalid value: 2: count doesn't match port range 80-82`,
			},
		},
		{
			"bosh-run-bad-ports.yml", []string{
				`roles[myrole].run.exposed-ports[https].internal: Invalid value: "-1": invalid syntax`,
				`roles[myrole].run.exposed-ports[https].external: Invalid value: 0: must be between 1 and 65535, inclusive`,
			},
		},
		{
			"bosh-run-missing-portrange.yml", []string{
				`roles[myrole].run.exposed-ports[https].internal: Invalid value: "": invalid syntax`,
			},
		},
		{
			"bosh-run-reverse-portrange.yml", []string{
				`roles[myrole].run.exposed-ports[https].internal: Invalid value: "5678-123": last port can't be lower than first port`,
			},
		},
		{
			"bosh-run-bad-parse.yml", []string{
				`roles[myrole].run.exposed-ports[https].internal: Invalid value: "qq": invalid syntax`,
				`roles[myrole].run.exposed-ports[https].external: Invalid value: "aa": invalid syntax`,
			},
		},
		{
			"bosh-run-bad-memory.yml", []string{
				`roles[myrole].run.memory: Invalid value: -10: must be greater than or equal to 0`,
			},
		},
		{
			"bosh-run-bad-cpu.yml", []string{
				`roles[myrole].run.virtual-cpus: Invalid value: -2: must be greater than or equal to 0`,
			},
		},
		{
			"bosh-run-env.yml", []string{
				`roles[xrole].run.env: Forbidden: Non-docker role declares bogus parameters`,
			},
		},
	}

	for _, tc := range tests {
		roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests", tc.manifest)
		roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
		if assert.Errorf(err, "Expected errors: %s", tc.message) {
			assert.Equal(tc.message, strings.Split(err.Error(), "\n"))
			assert.Nil(roleManifest)
		}
	}

	testsOk := []string{
		"exposed-ports.yml",
		"exposed-port-range.yml",
	}

	for _, manifest := range testsOk {
		roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests", manifest)
		_, err := LoadRoleManifest(roleManifestPath, []*Release{release}, nil)
		assert.Nil(err)
	}
}

func TestResolveLinks(t *testing.T) {
	workDir, err := os.Getwd()

	assert.NoError(t, err)

	var releases []*Release

	for _, dirName := range []string{"ntp-release", "tor-boshrelease"} {
		releasePath := filepath.Join(workDir, "../test-assets", dirName)
		releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
		release, err := NewDevRelease(releasePath, "", "", releasePathBoshCache)
		if assert.NoError(t, err) {
			releases = append(releases, release)
		}
	}

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/multiple-good.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, releases, nil)
	assert.NoError(t, err)

	// LoadRoleManifest implicitly runs resolveLinks()

	role := roleManifest.LookupRole("myrole")
	job := role.LookupJob("ntpd")
	if !assert.NotNil(t, job) {
		return
	}

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
				for name, consumeInfo := range job.ResolvedConsumers {
					assert.NotEqual(t, expected.Type, consumeInfo.Type,
						"link should not resolve, got %s (type %s) in %s / %s",
						name, consumeInfo.Type, consumeInfo.RoleName, consumeInfo.JobName)
				}
				return
			}
			expectedLength++
			require.Contains(t, job.ResolvedConsumers, expected.Name, "link %s is missing", expected.Name)
			actual := job.ResolvedConsumers[expected.Name]
			assert.Equal(t, expected.Name, actual.Name, "link name mismatch")
			assert.Equal(t, expected.Type, actual.Type, "link type mismatch")
			assert.Equal(t, role.Name, actual.RoleName, "link role name mismatch")
			assert.Equal(t, job.Name, actual.JobName, "link job name mismatch")
		})
	}
	assert.Len(t, job.ResolvedConsumers, expectedLength)
}

func TestRoleResolveLinksMultipleProvider(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	job1 := &Job{
		Name: "job-1",
		AvailableProviders: map[string]jobProvidesInfo{
			"job-1-provider-1": {
				jobLinkInfo: jobLinkInfo{
					Name: "job-1-provider-1",
					Type: "link-1",
				},
			},
			"job-1-provider-2": {
				jobLinkInfo: jobLinkInfo{
					Name: "job-1-provider-2",
					Type: "link-2",
				},
			},
			"job-1-provider-3": {
				jobLinkInfo: jobLinkInfo{
					Name: "job-1-provider-3",
					Type: "link-5",
				},
			},
		},
		DesiredConsumers: []jobConsumesInfo{
			{
				jobLinkInfo: jobLinkInfo{
					Name: "job-1-provider-1",
					Type: "link-1",
				},
			},
		},
	}

	job2 := &Job{
		Name: "job-2",
		AvailableProviders: map[string]jobProvidesInfo{
			"job-2-provider-1": {
				jobLinkInfo: jobLinkInfo{
					Name: "job-2-provider-1",
					Type: "link-3",
				},
			},
		},
	}

	job3 := &Job{
		Name: "job-3",
		AvailableProviders: map[string]jobProvidesInfo{
			"job-3-provider-3": {
				jobLinkInfo: jobLinkInfo{
					Name: "job-3-provider-3",
					Type: "link-4",
				},
			},
		},
		DesiredConsumers: []jobConsumesInfo{
			{
				// There is exactly one implicit provider of this type; use it
				jobLinkInfo: jobLinkInfo{
					Type: "link-1", // j1
				},
			},
			{
				// This job has multiple available implicit providers with
				// the same type; this should not resolve.
				jobLinkInfo: jobLinkInfo{
					Type: "link-3", // j3
				},
				Optional: true,
			},
			{
				// There is exactly one explicit provider of this name
				jobLinkInfo: jobLinkInfo{
					Name: "job-3-provider-3", // j3
				},
			},
			{
				// There are no providers of this type
				jobLinkInfo: jobLinkInfo{
					Type: "missing",
				},
				Optional: true,
			},
			{
				// This requires an alias
				jobLinkInfo: jobLinkInfo{
					Name: "actual-consumer-name",
				},
				Optional: true, // Not resolvable in role 3
			},
		},
	}

	roleManifest := &RoleManifest{
		Roles: Roles{
			&Role{
				Name: "role-1",
				RoleJobs: []*RoleJob{
					{
						Job: job1,
						ExportedProviders: map[string]jobProvidesInfo{
							"job-1-provider-3": jobProvidesInfo{
								Alias: "unique-alias",
							},
						},
					},
					{Job: job2},
				},
			},
			&Role{
				Name: "role-2",
				RoleJobs: []*RoleJob{
					{Job: job2},
					{
						Job: job3,
						// This has an explicitly exported provider
						ExportedProviders: map[string]jobProvidesInfo{
							"job-3-provider-3": jobProvidesInfo{},
						},
						ResolvedConsumers: map[string]jobConsumesInfo{
							"actual-consumer-name": jobConsumesInfo{
								Alias: "unique-alias",
							},
						},
					},
				},
			},
			&Role{
				Name: "role-3",
				// This does _not_ have an explicitly exported provider
				RoleJobs: []*RoleJob{{Job: job2}, {Job: job3}},
			},
		},
	}
	for _, r := range roleManifest.Roles {
		for _, roleJob := range r.RoleJobs {
			roleJob.Name = roleJob.Job.Name
			if roleJob.ResolvedConsumers == nil {
				roleJob.ResolvedConsumers = make(map[string]jobConsumesInfo)
			}
		}
	}
	errors := roleManifest.resolveLinks()
	assert.Empty(errors)
	role := roleManifest.LookupRole("role-2")
	require.NotNil(role, "Failed to find role")
	job := role.LookupJob("job-3")
	require.NotNil(job, "Failed to find job")
	consumes := job.ResolvedConsumers

	assert.Len(consumes, 3, "incorrect number of resulting link consumers")

	if assert.Contains(consumes, "job-1-provider-1", "failed to find role by type") {
		assert.Equal(jobConsumesInfo{
			jobLinkInfo: jobLinkInfo{
				Name:     "job-1-provider-1",
				Type:     "link-1",
				RoleName: "role-1",
				JobName:  "job-1",
			},
		}, consumes["job-1-provider-1"], "found incorrect role by type")
	}

	assert.NotContains(consumes, "job-3-provider-1",
		"should not automatically resolve consumers with multiple providers of the type")

	if assert.Contains(consumes, "job-3-provider-3", "did not find explicitly named provider") {
		assert.Equal(jobConsumesInfo{
			jobLinkInfo: jobLinkInfo{
				Name:     "job-3-provider-3",
				Type:     "link-4",
				RoleName: "role-2",
				JobName:  "job-3",
			},
		}, consumes["job-3-provider-3"], "did not find explicitly named provider")
	}

	if assert.Contains(consumes, "actual-consumer-name", "did not resolve consumer with alias") {
		assert.Equal(jobConsumesInfo{
			jobLinkInfo: jobLinkInfo{
				Name:     "job-1-provider-3",
				Type:     "link-5",
				RoleName: "role-1",
				JobName:  "job-1",
			},
		}, consumes["actual-consumer-name"], "resolved to incorrect provider for alias")
	}
}

func TestWriteConfigs(t *testing.T) {
	assert := assert.New(t)

	job := &Job{
		Name: "silly job",
		Properties: []*JobProperty{
			&JobProperty{
				Name:    "prop",
				Default: "bar",
			},
		},
		AvailableProviders: map[string]jobProvidesInfo{
			"<not used>": jobProvidesInfo{
				jobLinkInfo: jobLinkInfo{
					Name: "<not used>",
				},
				Properties: []string{"exported-prop"},
			},
		},
		DesiredConsumers: []jobConsumesInfo{
			jobConsumesInfo{
				jobLinkInfo: jobLinkInfo{
					Name: "serious",
					Type: "serious-type",
				},
			},
		},
	}

	role := &Role{
		Name: "dummy role",
		RoleJobs: []*RoleJob{
			{
				Job:  job,
				Name: "silly job",
				ResolvedConsumers: map[string]jobConsumesInfo{
					"serious": jobConsumesInfo{
						jobLinkInfo: jobLinkInfo{
							Name:     "serious",
							Type:     "serious-type",
							RoleName: "dummy role",
							JobName:  job.Name,
						},
					},
				},
			},
		},
	}

	tempFile, err := ioutil.TempFile("", "fissile-job-test")
	assert.NoError(err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(strings.Replace(`---
	properties:
		foo: 3
	`, "\t", "    ", -1))
	assert.NoError(err)
	assert.NoError(tempFile.Close())

	json, err := role.RoleJobs[0].WriteConfigs(role, tempFile.Name(), tempFile.Name())
	assert.NoError(err)

	assert.JSONEq(`
	{
		"job": {
			"name": "dummy role"
		},
		"parameters": {},
		"properties": {
			"prop": "bar"
		},
		"networks": {
			"default": {}
		},
		"exported_properties": [
			"prop"
		],
		"consumes": {
			"serious": {
				"role": "dummy role",
				"job": "silly job"
			}
		},
		"exported_properties": [
			"exported-prop"
		]
	}`, string(json))
}

func TestLoadRoleManifestColocatedContainers(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torRelease, err := NewDevRelease(torReleasePath, "", "", filepath.Join(torReleasePath, "bosh-cache"))
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", filepath.Join(ntpReleasePath, "bosh-cache"))
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/colocated-containers.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{torRelease, ntpRelease}, nil)
	assert.NoError(err)
	assert.NotNil(roleManifest)

	assert.Len(roleManifest.Roles, 2)
	assert.EqualValues(RoleTypeBosh, roleManifest.LookupRole("main-role").Type)
	assert.EqualValues(RoleTypeColocatedContainer, roleManifest.LookupRole("to-be-colocated").Type)
	assert.Len(roleManifest.LookupRole("main-role").ColocatedContainers, 1)

	for _, roleName := range []string{"main-role", "to-be-colocated"} {
		assert.EqualValues([]*RoleRunVolume{&RoleRunVolume{Path: "/var/vcap/store", Type: "emptyDir", Tag: "shared-data"}}, roleManifest.LookupRole(roleName).Run.Volumes)
	}
}

func TestLoadRoleManifestColocatedContainersValidationMissingRole(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torRelease, err := NewDevRelease(torReleasePath, "", "", filepath.Join(torReleasePath, "bosh-cache"))
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", filepath.Join(ntpReleasePath, "bosh-cache"))
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/colocated-containers-with-missing-role.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{torRelease, ntpRelease}, nil)
	assert.Nil(roleManifest)
	assert.EqualError(err, `roles[main-role].colocated_containers[0]: Invalid value: "to-be-colocated-typo": There is no such role defined`)
}

func TestLoadRoleManifestColocatedContainersValidationUsusedRole(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torRelease, err := NewDevRelease(torReleasePath, "", "", filepath.Join(torReleasePath, "bosh-cache"))
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", filepath.Join(ntpReleasePath, "bosh-cache"))
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/colocated-containers-with-unused-role.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{torRelease, ntpRelease}, nil)
	assert.Nil(roleManifest)
	assert.EqualError(err, "role[to-be-colocated].job[ntpd].consumes[ntp-server]: Required value: failed to resolve provider ntp-server (type ntpd)\n"+
		"role[orphaned].job[ntpd].consumes[ntp-server]: Required value: failed to resolve provider ntp-server (type ntpd)\n"+
		"role[orphaned]: Not found: \"role is of type colocated container, but is not used by any other role as such\"")
}

func TestLoadRoleManifestColocatedContainersValidationPortCollisions(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torRelease, err := NewDevRelease(torReleasePath, "", "", filepath.Join(torReleasePath, "bosh-cache"))
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", filepath.Join(ntpReleasePath, "bosh-cache"))
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/colocated-containers-with-port-collision.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{torRelease, ntpRelease}, nil)
	assert.Nil(roleManifest)
	assert.EqualError(err, "role[main-role]: Invalid value: 80: port collision, the same port is used by: main-role, to-be-colocated\n"+
		"role[main-role]: Invalid value: 10443: port collision, the same port is used by: main-role, to-be-colocated")
}

func TestLoadRoleManifestColocatedContainersValidationInvalidTags(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torRelease, err := NewDevRelease(torReleasePath, "", "", filepath.Join(torReleasePath, "bosh-cache"))
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", filepath.Join(ntpReleasePath, "bosh-cache"))
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/colocated-containers-with-clustered-tag.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{torRelease, ntpRelease}, nil)
	assert.Nil(roleManifest)
	assert.EqualError(err, "role[to-be-colocated]: Invalid value: \"clustered\": tags clustered, or indexed are not supported for colocated-containers")
}

func TestLoadRoleManifestColocatedContainersValidationOfSharedVolumes(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	torRelease, err := NewDevRelease(torReleasePath, "", "", filepath.Join(torReleasePath, "bosh-cache"))
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpRelease, err := NewDevRelease(ntpReleasePath, "", "", filepath.Join(ntpReleasePath, "bosh-cache"))
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/colocated-containers-with-volume-share-issues.yml")
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{torRelease, ntpRelease}, nil)
	assert.Nil(roleManifest)
	assert.EqualError(err, "role[to-be-colocated]: Invalid value: \"/mnt/foobAr\": colocated role specifies a shared volume with tag mount-share, which path does not match the path of the main role shared volume with the same tag\n"+
		"role[main-role]: Required value: container must use shared volumes of the main role: vcap-logs\n"+
		"role[main-role]: Required value: container must use shared volumes of the main role: vcap-store")
}
