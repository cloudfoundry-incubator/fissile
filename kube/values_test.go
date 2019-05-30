package kube

import (
	"testing"

	"code.cloudfoundry.org/fissile/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeValues(t *testing.T) {
	t.Parallel()

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

	t.Run("Ingress", func(t *testing.T) {
		t.Parallel()

		expected := `---

# ingress.annotations allows specifying custom ingress annotations that gets
# merged to the default annotations.
annotations: {}

# ingress.enabled enables ingress support - working ingress controller
# necessary.
enabled: false

# ingress.tls.crt and ingress.tls.key, when specified, are used by the TLS
# secret for the Ingress resource.
tls: {}
`

		settings := ExportSettings{
			RoleManifest: &model.RoleManifest{
				InstanceGroups: model.InstanceGroups{},
				Configuration:  &model.Configuration{},
			},
		}
		node := MakeValues(settings)
		require.NotNil(t, node)
		actual := node.Get("ingress").String()

		assert.Exactly(t, expected, actual)
	})
}
