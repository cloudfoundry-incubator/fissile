package kube

// ExportSettings are configuration for creating Kubernetes configs
type ExportSettings struct {
	Repository      string
	Defaults        map[string]string
	Registry        string
	Organization    string
	UseMemoryLimits bool
}
