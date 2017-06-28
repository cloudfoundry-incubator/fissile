package kube

import (
	"github.com/SUSE/fissile/model"
)

// ExportSettings are configuration for creating Kubernetes configs
type ExportSettings struct {
	Repository      string
	Defaults        map[string]string
	Registry        string
	Organization    string
	UseMemoryLimits bool
	FissileVersion  string
	Opinions        *model.Opinions
	Secrets         SecretRefMap
	CreateHelmChart bool
}
