package kube

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeValues(t *testing.T) {
	t.Parallel()

	outDir, err := ioutil.TempDir("", "fissile-generate-auth-")
	require.NoError(t, err)
	defer os.RemoveAll(outDir)

	t.Run("Capabilities", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{
				InstanceGroups: model.InstanceGroups{
					&model.InstanceGroup{
						Name: "arole",
						Run: &model.RoleRun{
							Scaling: &model.RoleRunScaling{},
						},
					},
				},
				Configuration: &model.Configuration{},
			},
		}

		node, err := MakeValues(settings)

		assert.NotNil(t, node)
		assert.NoError(t, err)

		actual, err := RoundtripKube(node)
		if !assert.NoError(t, err) {
			return
		}
		testhelpers.IsYAMLSubsetString(assert.New(t), `---
			sizing:
				arole:
					capabilities:	[]
		`, actual)
	})

	t.Run("Sizing", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{
				InstanceGroups: model.InstanceGroups{
					&model.InstanceGroup{
						Name: "arole",
						Run: &model.RoleRun{
							Scaling: &model.RoleRunScaling{},
						},
					},
				},
				Configuration: &model.Configuration{},
			},
		}

		node, err := MakeValues(settings)
		assert.NoError(t, err)
		require.NotNil(t, node)

		sizing := node.Get("sizing")
		require.NotNil(t, sizing)
		assert.Contains(t, sizing.Comment(), "underscore")
	})

	t.Run("Check Default Registry", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{InstanceGroups: model.InstanceGroups{},
				Configuration: &model.Configuration{},
			},
		}

		node, err := MakeValues(settings)

		assert.NotNil(t, node)
		assert.NoError(t, err)

		registry := node.Get("kube").Get("registry").Get("hostname")

		assert.Equal(t, registry.String(), "docker.io")
	})

	t.Run("Check Custom Registry", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{InstanceGroups: model.InstanceGroups{},
				Configuration: &model.Configuration{},
			},
		}

		settings.Registry = "example.com"

		node, err := MakeValues(settings)

		assert.NotNil(t, node)
		assert.NoError(t, err)

		registry := node.Get("kube").Get("registry").Get("hostname")

		assert.Equal(t, registry.String(), "example.com")
	})

	t.Run("Check Default Auth", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{InstanceGroups: model.InstanceGroups{},
				Configuration: &model.Configuration{},
			},
		}

		node, err := MakeValues(settings)

		assert.NotNil(t, node)
		assert.NoError(t, err)

		auth := node.Get("kube").Get("auth")

		assert.Equal(t, auth.String(), "~", "Default value should be nil")
	})

	t.Run("Check Custom Auth", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{InstanceGroups: model.InstanceGroups{},
				Configuration: &model.Configuration{},
			},
		}

		authString := "foo"

		settings.AuthType = authString

		node, err := MakeValues(settings)

		assert.NotNil(t, node)
		assert.NoError(t, err)

		auth := node.Get("kube").Get("auth")

		assert.Equal(t, auth.String(), authString)
	})
}
