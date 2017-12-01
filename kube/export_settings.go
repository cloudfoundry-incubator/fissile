package kube

import (
	"github.com/SUSE/fissile/model"
)

// ExportSettings are configuration for creating Kubernetes configs
type ExportSettings struct {
	OutputDir           string
	Repository          string
	Defaults            map[string]string
	Registry            string
	Username            string
	Password            string
	Organization        string
	UseMemoryLimits     bool
	FissileVersion      string
	TagExtra            string
	RoleManifest        *model.RoleManifest
	Opinions            *model.Opinions
	Secrets             SecretRefMap
	CreateHelmChart     bool
	UseSecretsGenerator bool
}
