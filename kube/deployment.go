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
	spec.AddNode("selector", newSelector(role.Name))
	spec.AddNode("template", podTemplate)

	deployment := newKubeConfig("extensions/v1beta1", "Deployment", role.Name, helm.Comment(role.GetLongDescription()))
	replicaCheck(role, deployment, spec, svc, settings)
	deployment.AddNode("spec", spec.Sort())

	return deployment.Sort(), svc, nil
}

func replicaCheck(role *model.Role, controller *helm.Mapping, spec *helm.Mapping, service helm.Node, settings *ExportSettings) {
	if !settings.CreateHelmChart {
		spec.AddInt("replicas", role.Run.Scaling.Min)
		return
	}

	roleName := makeVarName(role.Name)
	spec.Add("replicas", fmt.Sprintf("{{ .Values.sizing.%s.count }}", roleName))
	if role.Run.Scaling.Min == 0 {
		block := helm.Block(fmt.Sprintf("if gt (int .Values.sizing.%s.count) 0", roleName))
		controller.Set(block)
		if service != nil {
			service.Set(block)
		}
	} else {
		fail := fmt.Sprintf(`{{ fail "%s must have at least %d instances" }}`, roleName, role.Run.Scaling.Min)
		block := fmt.Sprintf("if lt (int .Values.sizing.%s.count) %d", roleName, role.Run.Scaling.Min)
		controller.Add("_minReplicas", fail, helm.Block(block))
	}

	fail := fmt.Sprintf(`{{ fail "%s cannot have more than %d instances" }}`, roleName, role.Run.Scaling.Max)
	block := fmt.Sprintf("if gt (int .Values.sizing.%s.count) %d", roleName, role.Run.Scaling.Max)
	controller.Add("_maxReplicas", fail, helm.Block(block))
}
