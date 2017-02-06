package kube

type KubeExportSettings struct {
	Repository   string
	Defaults     map[string]string
	Registry     string
	Organization string
}
