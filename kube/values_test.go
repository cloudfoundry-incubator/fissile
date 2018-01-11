package kube

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeValues(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	outDir, err := ioutil.TempDir("", "fissile-generate-auth-")
	require.NoError(t, err)
	defer os.RemoveAll(outDir)

	t.Run("Check Default Registry", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{Roles: model.Roles{},
				Configuration: &model.Configuration{},
			},
		}

		node, err := MakeValues(settings)

		assert.NotNil(node)
		assert.NoError(err)

		registry := node.Get("kube").Get("registry").Get("hostname")

		assert.Equal(registry.String(), "docker.io")
	})

	t.Run("Check Custom Registry", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{Roles: model.Roles{},
				Configuration: &model.Configuration{},
			},
		}

		settings.Registry = "example.com"

		node, err := MakeValues(settings)

		assert.NotNil(node)
		assert.NoError(err)

		registry := node.Get("kube").Get("registry").Get("hostname")

		assert.Equal(registry.String(), "example.com")
	})

	t.Run("Check Default Auth", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{Roles: model.Roles{},
				Configuration: &model.Configuration{},
			},
		}

		node, err := MakeValues(settings)

		assert.NotNil(node)
		assert.NoError(err)

		auth := node.Get("kube").Get("auth")

		assert.Equal(auth.String(), "~", "Default value should be nil")
	})

	t.Run("Check Custom Auth", func(t *testing.T) {
		t.Parallel()
		settings := ExportSettings{
			OutputDir: outDir,
			RoleManifest: &model.RoleManifest{Roles: model.Roles{},
				Configuration: &model.Configuration{},
			},
		}

		authString := "foo"

		settings.AuthType = authString

		node, err := MakeValues(settings)

		assert.NotNil(node)
		assert.NoError(err)

		auth := node.Get("kube").Get("auth")

		assert.Equal(auth.String(), authString)
	})
}
