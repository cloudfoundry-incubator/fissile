package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestRolesSort(t *testing.T) {
	assert := assert.New(t)

	instanceGroups := InstanceGroups{
		{Name: "aaa"},
		{Name: "bbb"},
	}
	sort.Sort(instanceGroups)
	assert.Equal(instanceGroups[0].Name, "aaa")
	assert.Equal(instanceGroups[1].Name, "bbb")

	instanceGroups = InstanceGroups{
		{Name: "ddd"},
		{Name: "ccc"},
	}
	sort.Sort(instanceGroups)
	assert.Equal(instanceGroups[0].Name, "ccc")
	assert.Equal(instanceGroups[1].Name, "ddd")
}

func TestGetScriptPaths(t *testing.T) {
	workDir, err := os.Getwd()
	assert.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/model/tor-good.yml")
	roleManifest, err := LoadRoleManifest(
		roleManifestPath,
		[]string{torReleasePath},
		[]string{},
		[]string{},
		filepath.Join(workDir, "../test-assets/bosh-cache"),
		nil)
	assert.NoError(t, err)
	require.NotNil(t, roleManifest)

	fullScripts := roleManifest.InstanceGroups[0].GetScriptPaths()
	assert.Len(t, fullScripts, 3)
	for _, leafName := range []string{"environ.sh", "myrole.sh", "post_config_script.sh"} {
		assert.Equal(t, filepath.Join(workDir, "../test-assets/role-manifests/model", leafName), fullScripts[leafName])
	}
}

func TestGetScriptSignatures(t *testing.T) {
	assert := assert.New(t)

	refRole := &InstanceGroup{
		Name: "bbb",
		JobReferences: JobReferences{
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

	differentPatch := &InstanceGroup{
		Name:          refRole.Name,
		JobReferences: JobReferences{refRole.JobReferences[0], refRole.JobReferences[1]},
		Scripts:       []string{scriptName},
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

	differentTemplate1 := &InstanceGroup{
		Name:          "aaa",
		JobReferences: JobReferences{},
		Configuration: &Configuration{
			Templates: yaml.MapSlice{
				yaml.MapItem{
					Key:   "foo",
					Value: "bar",
				}}},
	}

	differentTemplate2 := &InstanceGroup{
		Name:          "aaa",
		JobReferences: JobReferences{},
		Configuration: &Configuration{
			Templates: yaml.MapSlice{
				yaml.MapItem{
					Key:   "bat",
					Value: "baz",
				}}},
	}

	differentTemplateHash1, _ := differentTemplate1.GetTemplateSignatures()
	differentTemplateHash2, _ := differentTemplate2.GetTemplateSignatures()
	assert.NotEqual(differentTemplateHash1, differentTemplateHash2, "template hash should be dependent on template contents")
}
