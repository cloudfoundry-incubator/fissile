package kube

import (
	"code.cloudfoundry.org/fissile/model"
)

// ExportSettings are configuration for creating Kubernetes configs
type ExportSettings struct {
	OutputDir       string
	Repository      string
	Registry        string
	Username        string
	Password        string
	Organization    string
	UseMemoryLimits bool
	UseCPULimits    bool
	FissileVersion  string
	TagExtra        string
	RoleManifest    *model.RoleManifest
	Opinions        *model.Opinions
	CreateHelmChart bool
	AuthType        string
	IstioComplied   bool
}
