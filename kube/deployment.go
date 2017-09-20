package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// NewDeployment creates a Deployment for the given role, and its attached services
func NewDeployment(role *model.Role, settings *ExportSettings) (helm.Node, helm.Node, error) {
	podTemplate, err := NewPodTemplate(role, settings)
	if err != nil {
		return nil, nil, err
	}

	svc, err := NewClusterIPServiceList(role, false, settings)
	if err != nil {
		return nil, nil, err
	}

	spec := helm.NewEmptyMapping()
	if settings.CreateHelmChart {
		spec.Add("replicas", fmt.Sprintf("{{ .Values.sizing.%s.count }}", makeVarName(role.Name)))
	} else {
		spec.AddInt("replicas", role.Run.Scaling.Min)
	}
	spec.AddNode("selector", newSelector(role.Name))
	spec.AddNode("template", podTemplate)

	deployment := newKubeConfig("extensions/v1beta1", "Deployment", role.Name, helm.Comment(role.GetLongDescription()))
	deployment.AddNode("spec", spec)

	return deployment.Sort(), svc, nil
}
