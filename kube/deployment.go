package kube

import (
	"errors"
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"
)

// NewDeployment creates a Deployment for the given role, and its attached services
func NewDeployment(role *model.Role, settings ExportSettings, grapher util.ModelGrapher) (helm.Node, helm.Node, error) {
	podTemplate, err := NewPodTemplate(role, settings, grapher)
	if err != nil {
		return nil, nil, err
	}

	svc, err := NewClusterIPServiceList(role, false, true, settings)
	if err != nil {
		return nil, nil, err
	}
	spec := helm.NewMapping()
	spec.Add("selector", newSelector(role.Name))
	spec.Add("template", podTemplate)

	deployment := newKubeConfig("extensions/v1beta1", "Deployment", role.Name, helm.Comment(role.GetLongDescription()))
	deployment.Add("spec", spec)
	err = replicaCheck(role, deployment, svc, settings)
	return deployment, svc, err
}

// getAffinityBlock returns an affinity block to add to a podspec
func getAffinityBlock(role *model.Role) *helm.Mapping {
	affinity := helm.NewMapping()

	if role.Run.Affinity.PodAntiAffinity != nil {
		// Add pod anti affinity from role manifest
		affinity.Add("podAntiAffinity", role.Run.Affinity.PodAntiAffinity)
	}

	// Add node affinity template to be filled in by values.yaml
	roleName := makeVarName(role.Name)
	nodeCond := fmt.Sprintf("if .Values.sizing.%s.affinity.nodeAffinity", roleName)
	nodeAffinity := fmt.Sprintf("{{ toJson .Values.sizing.%s.affinity.nodeAffinity }}", roleName)
	affinity.Add("nodeAffinity", nodeAffinity, helm.Block(nodeCond))

	return affinity
}

// getAffinityAnnotation returns a Kubernetes 1.5 affinity annotation to add to a podspec
func getAffinityAnnotation(role *model.Role) (string, error) {
	affinitySpec := make(map[string]interface{})
	if role.Run.Affinity.PodAntiAffinity != nil {
		affinitySpec["podAntiAffinity"] = role.Run.Affinity.PodAntiAffinity
	}
	affinity, err := util.JSONMarshal(affinitySpec)
	if err != nil {
		return "", err
	}
	return string(affinity), nil
}

// addAffinityRules adds affinity rules to the pod spec
func addAffinityRules(role *model.Role, spec *helm.Mapping, settings ExportSettings) error {
	if role.Run.Affinity != nil {
		if role.Run.Affinity.NodeAffinity != nil {
			return errors.New("node affinity in role manifest not allowed")
		}

		if role.Run.Affinity.PodAffinity != nil {
			return errors.New("pod affinity in role manifest not supported")
		}

		var cond string
		if settings.CreateHelmChart {
			podSpec := spec.Get("template", "spec").(*helm.Mapping)

			// affinity spec is only supported in kube 1.6 and later
			cond = fmt.Sprintf("if %s", minKubeVersion(1, 6))

			podSpec.Add("affinity", getAffinityBlock(role), helm.Block(cond))
			podSpec.Sort()

			// in kube 1.5 affinity is declared via annotation
			cond = fmt.Sprintf("if not (%s)", minKubeVersion(1, 6))
		}

		meta := spec.Get("template", "metadata").(*helm.Mapping)
		if meta.Get("annotations") == nil {
			meta.Add("annotations", helm.NewMapping())
			meta.Sort()
		}
		annotations := meta.Get("annotations").(*helm.Mapping)

		affinity, err := getAffinityAnnotation(role)

		if err != nil {
			return err
		}

		annotations.Add("scheduler.alpha.kubernetes.io/affinity", affinity, helm.Block(cond))
		annotations.Sort()
	}

	return nil
}

func replicaCheck(role *model.Role, controller *helm.Mapping, service helm.Node, settings ExportSettings) error {
	spec := controller.Get("spec").(*helm.Mapping)

	err := addAffinityRules(role, spec, settings)
	if err != nil {
		return err
	}

	if !settings.CreateHelmChart {
		spec.Add("replicas", role.Run.Scaling.Min)
		spec.Sort()
		return nil
	}

	roleName := makeVarName(role.Name)
	count := fmt.Sprintf(".Values.sizing.%s.count", roleName)
	if role.Run.Scaling.HA != role.Run.Scaling.Min {
		// Under HA use HA count if the user hasn't explicitly modified the default count
		count = fmt.Sprintf("{{ if and .Values.sizing.HA (eq (int %s) %d) -}} %d {{- else -}} {{ %s }} {{- end }}",
			count, role.Run.Scaling.Min, role.Run.Scaling.HA, count)
	} else {
		count = "{{ " + count + " }}"
	}
	spec.Add("replicas", count)
	spec.Sort()

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

		if role.Run.Scaling.HA != role.Run.Scaling.Min {
			fail := fmt.Sprintf(`{{ fail "%s must have at least %d instances for HA" }}`, roleName, role.Run.Scaling.HA)
			count := fmt.Sprintf(".Values.sizing.%s.count", roleName)
			// If count != Min then count must be >= HA
			block := fmt.Sprintf("if and .Values.sizing.HA (and (ne (int %s) %d) (lt (int %s) %d))",
				count, role.Run.Scaling.Min, count, role.Run.Scaling.HA)
			controller.Add("_minHAReplicas", fail, helm.Block(block))
		}
	}

	fail := fmt.Sprintf(`{{ fail "%s cannot have more than %d instances" }}`, roleName, role.Run.Scaling.Max)
	block := fmt.Sprintf("if gt (int .Values.sizing.%s.count) %d", roleName, role.Run.Scaling.Max)
	controller.Add("_maxReplicas", fail, helm.Block(block))

	if role.Run.Scaling.MustBeOdd {
		fail := fmt.Sprintf(`{{ fail "%s must have an odd instance count" }}`, roleName)
		block := fmt.Sprintf("if eq (mod (int .Values.sizing.%s.count) 2) 0", roleName)
		controller.Add("_oddReplicas", fail, helm.Block(block))
	}

	controller.Sort()

	return nil
}
