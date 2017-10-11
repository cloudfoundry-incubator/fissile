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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	torjob := foorole.Jobs[0]
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	_, err = LoadRoleManifest(roleManifestPath, []*Release{release})
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
	_, err = LoadRoleManifest(roleManifestPath, []*Release{release, release})

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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{ntpRelease, torRelease})
	assert.NoError(err)
	assert.NotNil(roleManifest)

	assert.Equal(roleManifestPath, roleManifest.manifestFilePath)
	assert.Len(roleManifest.Roles, 2)

	myrole := roleManifest.Roles[0]
	assert.Len(myrole.Scripts, 1)
	assert.Equal("myrole.sh", myrole.Scripts[0])

	foorole := roleManifest.Roles[1]
	torjob := foorole.Jobs[0]
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
	_, err = LoadRoleManifest(roleManifestPath, []*Release{ntpRelease, torRelease})

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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
	assert.NoError(err)
	assert.NotNil(roleManifest)

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
		Jobs: Jobs{
			{
				SHA1: "Role 2 Job 1",
				Packages: Packages{
					{Name: "aaa", SHA1: "Role 2 Job 1 Package 1"},
					{Name: "bbb", SHA1: "Role 2 Job 1 Package 2"},
				},
			},
			{
				SHA1: "Role 2 Job 2",
				Packages: Packages{
					{Name: "ccc", SHA1: "Role 2 Job 2 Package 1"},
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
		Name:    refRole.Name,
		Jobs:    Jobs{refRole.Jobs[0], refRole.Jobs[1]},
		Scripts: []string{scriptName},
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
		Name: "aaa",
		Jobs: Jobs{},
		Configuration: &Configuration{
			Templates: map[string]string{"foo": "bar"},
		},
	}

	differentTemplate2 := &Role{
		Name: "aaa",
		Jobs: Jobs{},
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})

	assert.Contains(err.Error(), `configuration.variables: Invalid value: "FOO": Does not sort before 'BAR'`)
	assert.Contains(err.Error(), `configuration.variables: Invalid value: "PELERINUL": Does not sort before 'ALPHA'`)
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
	assert.Equal(err.Error(),
		`roles[dockerrole].run.env: Not found: "No variable declaration of 'UNKNOWN'"`)
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
			"bosh-run-bad-ports.yml", []string{
				`roles[myrole].run.exposed-ports[https].external: Invalid value: 0: must be between 1 and 65535, inclusive`,
				`roles[myrole].run.exposed-ports[https].internal: Invalid value: "-1": invalid syntax`,
			},
		},
		{
			"bosh-run-bad-parse.yml", []string{
				`roles[myrole].run.exposed-ports[https].external: Invalid value: "aa": invalid syntax`,
				`roles[myrole].run.exposed-ports[https].internal: Invalid value: "qq": invalid syntax`,
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
		roleManifest, err := LoadRoleManifest(roleManifestPath, []*Release{release})
		assert.Equal(tc.message, strings.Split(err.Error(), "\n"))
		assert.Nil(roleManifest)
	}

	testsOk := []string{
		"exposed-ports.yml",
		"exposed-port-range.yml",
	}

	for _, manifest := range testsOk {
		roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests", manifest)
		_, err := LoadRoleManifest(roleManifestPath, []*Release{release})
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
	roleManifest, err := LoadRoleManifest(roleManifestPath, releases)
	assert.NoError(t, err)

	errList := roleManifest.resolveLinks()
	assert.Empty(t, errList)

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
		{Name: "ntp-client", Type: "ntp", Optional: true},
		{Type: "missing", Optional: true, Missing: true},
	}

	if !assert.Len(t, job.LinkConsumes, len(samples)) {
		return
	}
	for i, expected := range samples {
		t.Run("", func(t *testing.T) {
			actual := role.jobConsumes[job.Name][i]
			assert.Equal(t, expected.Name, actual.Name)
			assert.Equal(t, expected.Type, actual.Type)
			assert.Equal(t, expected.Optional, actual.Optional)
			if expected.Missing {
				assert.Empty(t, actual.RoleName)
				assert.Empty(t, actual.JobName)
			} else {
				assert.Equal(t, role.Name, actual.RoleName)
				assert.Equal(t, job.Name, actual.JobName)
			}
		})
	}
}

func TestRoleResolveLinksMultipleProvider(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	job1 := Job{
		Name: "job-1",
		LinkProvides: []*JobLinkProvides{
			&JobLinkProvides{
				Name: "job-1-provider-1",
				Type: "link-1",
			},
			&JobLinkProvides{
				Name: "job-1-provider-2",
				Type: "link-2",
			},
		},
		LinkConsumes: []*JobLinkConsumes{
			&JobLinkConsumes{
				Name: "job-1-provider-1",
				Type: "link-1",
			},
		},
	}

	job2 := Job{
		Name: "job-2",
		LinkProvides: []*JobLinkProvides{
			&JobLinkProvides{
				Name: "job-2-provider-1",
				Type: "link-3",
			},
		},
	}

	job3 := Job{
		Name: "job-3",
		LinkProvides: []*JobLinkProvides{
			&JobLinkProvides{
				Name: "job-3-provider-1",
				Type: "link-2",
			},
			&JobLinkProvides{
				Name: "job-3-provider-3",
				Type: "link-4",
			},
		},
		LinkConsumes: []*JobLinkConsumes{
			&JobLinkConsumes{
				Type: "link-1", // j1
			},
			&JobLinkConsumes{
				Type: "link-2", // j3
			},
			&JobLinkConsumes{
				Type: "link-3", // j3
			},
			&JobLinkConsumes{
				Name: "job-3-provider-3", // j3
			},
			&JobLinkConsumes{
				Type:     "missing",
				Optional: true,
			},
		},
	}

	roleManifest := &RoleManifest{
		Roles: Roles{
			&Role{
				Name: "role-1",
				Jobs: Jobs{&job1, &job2},
			},
			&Role{
				Name: "role-2",
				Jobs: Jobs{&job2, &job3},
			},
			&Role{
				Name: "role-3",
				Jobs: Jobs{&job2, &job3},
			},
		},
	}
	errors := roleManifest.resolveLinks()
	assert.Empty(errors)
	role := roleManifest.LookupRole("role-2")
	require.NotNil(role, "Failed to find role")
	consumes := role.jobConsumes["job-3"]

	require.Len(consumes, 5, "incorrect number of resulting link consumers")

	assert.Equal(JobLinkConsumes{
		Name:     "job-1-provider-1",
		Type:     "link-1",
		RoleName: "role-1",
		JobName:  "job-1",
	}, consumes[0], "failed to find role by type")

	assert.Equal(JobLinkConsumes{
		Name:     "job-3-provider-1",
		Type:     "link-2",
		RoleName: "role-2",
		JobName:  "job-3",
	}, consumes[1], "did not prefer providers in same role+job")

	// Role 2 and role 3 have the same set of jobs, but role 3 should not affect what role 2 links to
	assert.Equal(JobLinkConsumes{
		Name:     "job-2-provider-1",
		Type:     "link-3",
		RoleName: "role-2",
		JobName:  "job-2",
	}, consumes[2], "did not prefer providers in same role")

	assert.Equal(JobLinkConsumes{
		Name:     "job-3-provider-3",
		Type:     "link-4",
		RoleName: "role-2",
		JobName:  "job-3",
	}, consumes[3], "did not find explicitly named provider")

	assert.Empty(consumes[4].Name, "should not have resolved unresolvable consumer")
}
