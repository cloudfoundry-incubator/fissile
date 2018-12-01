package kube

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
)

// NewDeployment creates a Deployment for the given instance group, and its attached services
func NewDeployment(instanceGroup *model.InstanceGroup, settings ExportSettings, grapher util.ModelGrapher) (helm.Node, helm.Node, error) {
	podTemplate, err := NewPodTemplate(instanceGroup, settings, grapher)
	if err != nil {
		return nil, nil, err
	}

	svc, err := NewServiceList(instanceGroup, false, settings)
	if err != nil {
		return nil, nil, err
	}
	spec := helm.NewMapping()
	spec.Add("selector", newSelector(instanceGroup, settings))
	spec.Add("template", podTemplate)

	deployment := newKubeConfig(settings, "extensions/v1beta1", "Deployment", instanceGroup.Name, helm.Comment(instanceGroup.GetLongDescription()))
	deployment.Add("spec", spec)
	err = replicaCheck(instanceGroup, deployment, svc, settings)
	if err != nil {
		return nil, nil, err
	}
	err = generalCheck(instanceGroup, deployment, settings)
	return deployment, svc, err
}

// getAffinityBlock returns an affinity block to add to a podspec
func getAffinityBlock(instanceGroup *model.InstanceGroup) *helm.Mapping {
	affinity := helm.NewMapping()

	if instanceGroup.Run != nil && instanceGroup.Run.Affinity != nil && instanceGroup.Run.Affinity.PodAntiAffinity != nil {
		// Add pod anti affinity from role manifest
		affinity.Add("podAntiAffinity", instanceGroup.Run.Affinity.PodAntiAffinity)
	}

	// Add node affinity template to be filled in by values.yaml
	roleName := makeVarName(instanceGroup.Name)
	nodeCond := fmt.Sprintf("if .Values.sizing.%s.affinity.nodeAffinity", roleName)
	nodeAffinity := fmt.Sprintf("{{ toJson .Values.sizing.%s.affinity.nodeAffinity }}", roleName)
	affinity.Add("nodeAffinity", nodeAffinity, helm.Block(nodeCond))

	return affinity
}

// addAffinityRules adds affinity rules to the pod spec
func addAffinityRules(instanceGroup *model.InstanceGroup, spec *helm.Mapping, settings ExportSettings) error {
	if instanceGroup.Run.Affinity != nil {
		if instanceGroup.Run.Affinity.NodeAffinity != nil {
			return errors.New("node affinity in role manifest not allowed")
		}

		if instanceGroup.Run.Affinity.PodAffinity != nil {
			return errors.New("pod affinity in role manifest not supported")
		}
	}

	if settings.CreateHelmChart {
		podSpec := spec.Get("template", "spec").(*helm.Mapping)

		podSpec.Add("affinity", getAffinityBlock(instanceGroup))
		podSpec.Sort()
	}

	meta := spec.Get("template", "metadata").(*helm.Mapping)
	if meta.Get("annotations") == nil {
		meta.Add("annotations", helm.NewMapping())
		meta.Sort()
	}
	annotations := meta.Get("annotations").(*helm.Mapping)

	annotations.Sort()

	return nil
}

// generalCheck adds common guards to the pod described by the
// controller. This only applies to helm charts, not basic kube
// definitions.
func generalCheck(instanceGroup *model.InstanceGroup, controller *helm.Mapping, settings ExportSettings) error {
	if !settings.CreateHelmChart {
		return nil
	}

	// The global config keys found under `sizing` in
	// `values.yaml` (HA, cpu, memory) were moved out of that
	// hierarchy into `config`. This gives `sizing` a uniform
	// structure, containing only the per-instance-group descriptions. It
	// also means that we now have to guard ourselves against use
	// of the old keys. Here we add the necessary guard
	// conditions.
	//
	// Note. The construction shown for requests and limits is used because of
	// (1) When FOO is nil you cannot check for FOO.BAR, for any BAR.
	// (2) The `and` operator is not short cuircuited, it evals all of its arguments
	// Thus `and FOO FOO.BAR` will not work either

	fail := `{{ fail "Bad use of moved variable sizing.HA. The new name to use is config.HA" }}`
	controller.Add("_moved_sizing_HA", fail, helm.Block("if .Values.sizing.HA"))

	for _, key := range []string{
		"cpu",
		"memory",
	} {
		// requests, limits - More complex to avoid limitations of the go templating system.
		// Guard on the main variable and then use a guarded value for the child.
		// The else branch is present in case we happen to get instance groups named `cpu` or `memory`.

		for _, subkey := range []string{
			"limits",
			"requests",
		} {
			guardVariable := fmt.Sprintf("_moved_sizing_%s_%s", key, subkey)
			block := fmt.Sprintf("if .Values.sizing.%s", key)
			fail := fmt.Sprintf(`{{ if .Values.sizing.%s.%s }} {{ fail "Bad use of moved variable sizing.%s.%s. The new name to use is config.%s.%s" }} {{else}} ok {{end}}`,
				key, subkey, key, subkey, key, subkey)
			controller.Add(guardVariable, fail, helm.Block(block))
		}
	}

	controller.Sort()
	return nil
}

// replicaCheck adds various guards to validate the number of replicas
// for the pod described by the controller. It further adds the
// replicas specification itself as well.
func replicaCheck(instanceGroup *model.InstanceGroup, controller *helm.Mapping, service helm.Node, settings ExportSettings) error {
	spec := controller.Get("spec").(*helm.Mapping)

	err := addAffinityRules(instanceGroup, spec, settings)
	if err != nil {
		return err
	}

	if !settings.CreateHelmChart {
		spec.Add("replicas", instanceGroup.Run.Scaling.Min)
		spec.Sort()
		return nil
	}

	roleName := makeVarName(instanceGroup.Name)
	count := fmt.Sprintf(".Values.sizing.%s.count", roleName)
	if instanceGroup.Run.Scaling.HA != instanceGroup.Run.Scaling.Min {
		// Under HA use HA count if the user hasn't explicitly modified the default count
		count = fmt.Sprintf("{{ if and .Values.config.HA (eq (int %s) %d) -}} %d {{- else -}} {{ %s }} {{- end }}",
			count, instanceGroup.Run.Scaling.Min, instanceGroup.Run.Scaling.HA, count)
	} else {
		count = "{{ " + count + " }}"
	}
	spec.Add("replicas", count)
	spec.Sort()

	if instanceGroup.Run.Scaling.Min == 0 {
		block := helm.Block(fmt.Sprintf("if gt (int .Values.sizing.%s.count) 0", roleName))
		controller.Set(block)
		if service != nil {
			service.Set(block)
		}
	} else {
		fail := fmt.Sprintf(`{{ fail "%s must have at least %d instances" }}`, roleName, instanceGroup.Run.Scaling.Min)
		block := fmt.Sprintf("if lt (int .Values.sizing.%s.count) %d", roleName, instanceGroup.Run.Scaling.Min)
		controller.Add("_minReplicas", fail, helm.Block(block))

		if instanceGroup.Run.Scaling.HA != instanceGroup.Run.Scaling.Min {
			fail := fmt.Sprintf(`{{ fail "%s must have at least %d instances for HA" }}`, roleName, instanceGroup.Run.Scaling.HA)
			count := fmt.Sprintf(".Values.sizing.%s.count", roleName)
			// If count != Min then count must be >= HA
			block := fmt.Sprintf("if and .Values.config.HA (and (ne (int %s) %d) (lt (int %s) %d))",
				count, instanceGroup.Run.Scaling.Min, count, instanceGroup.Run.Scaling.HA)
			controller.Add("_minHAReplicas", fail, helm.Block(block))
		}
	}

	fail := fmt.Sprintf(`{{ fail "%s cannot have more than %d instances" }}`, roleName, instanceGroup.Run.Scaling.Max)
	block := fmt.Sprintf("if gt (int .Values.sizing.%s.count) %d", roleName, instanceGroup.Run.Scaling.Max)
	controller.Add("_maxReplicas", fail, helm.Block(block))

	if instanceGroup.Run.Scaling.MustBeOdd {
		fail := fmt.Sprintf(`{{ fail "%s must have an odd instance count" }}`, roleName)
		block := fmt.Sprintf("if eq (mod (int .Values.sizing.%s.count) 2) 0", roleName)
		controller.Add("_oddReplicas", fail, helm.Block(block))
	}

	controller.Sort()

	return nil
}
