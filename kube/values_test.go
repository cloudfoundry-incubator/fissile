package kube

import (
	"testing"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeValues(t *testing.T) {
	t.Parallel()

	t.Run("Capabilities", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
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

		node := MakeValues(settings)
		require.NotNil(t, node)

		actual, err := RoundtripKube(node)
		require.NoError(t, err)
		testhelpers.IsYAMLSubsetString(assert.New(t), `---
			sizing:
				arole:
					capabilities:	[]
		`, actual)
	})

	t.Run("Sizing", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
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

		node := MakeValues(settings)
		require.NotNil(t, node)

		sizing := node.Get("sizing")
		require.NotNil(t, sizing)
		assert.Contains(t, sizing.Comment(), "underscore")
	})

	t.Run("Check Default Registry", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			RoleManifest: &model.RoleManifest{
				InstanceGroups: model.InstanceGroups{},
				Configuration:  &model.Configuration{},
			},
		}

		node := MakeValues(settings)
		require.NotNil(t, node)

		registry := node.Get("kube").Get("registry").Get("hostname")

		assert.Equal(t, registry.String(), "docker.io")
	})

	t.Run("Check Custom Registry", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			RoleManifest: &model.RoleManifest{
				InstanceGroups: model.InstanceGroups{},
				Configuration:  &model.Configuration{},
			},
		}

		settings.Registry = "example.com"

		node := MakeValues(settings)
		require.NotNil(t, node)

		registry := node.Get("kube").Get("registry").Get("hostname")

		assert.Equal(t, registry.String(), "example.com")
	})

	t.Run("Check Default Auth", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			RoleManifest: &model.RoleManifest{
				InstanceGroups: model.InstanceGroups{},
				Configuration:  &model.Configuration{},
			},
		}

		node := MakeValues(settings)
		require.NotNil(t, node)

		auth := node.Get("kube").Get("auth")

		assert.Equal(t, auth.String(), "~", "Default value should be nil")
	})

	t.Run("Check Custom Auth", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			RoleManifest: &model.RoleManifest{
				InstanceGroups: model.InstanceGroups{},
				Configuration:  &model.Configuration{},
			},
		}

		authString := "foo"

		settings.AuthType = authString

		node := MakeValues(settings)
		require.NotNil(t, node)

		auth := node.Get("kube").Get("auth")

		assert.Equal(t, auth.String(), authString)
	})
}
