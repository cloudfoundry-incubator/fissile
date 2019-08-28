package kube

import (
	"fmt"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
)

// NewJob creates a new Job for the given instance group, as well as any objects it depends on
func NewJob(instanceGroup *model.InstanceGroup, settings ExportSettings, grapher util.ModelGrapher) (helm.Node, error) {
	podTemplate, err := NewPodTemplate(instanceGroup, settings, grapher)
	if err != nil {
		return nil, err
	}

	// Jobs must have a restart policy that isn't "always"
	switch instanceGroup.Run.FlightStage {
	case model.FlightStageManual:
		podTemplate.Get("spec", "restartPolicy").SetValue("Never")
	case model.FlightStageFlight, model.FlightStagePreFlight, model.FlightStagePostFlight:
		podTemplate.Get("spec", "restartPolicy").SetValue("OnFailure")
	default:
		return nil, fmt.Errorf("Instance group %s has unexpected flight stage %s", instanceGroup.Name, instanceGroup.Run.FlightStage)
	}

	name := instanceGroup.Name
	if settings.CreateHelmChart {
		name += "-{{ .Release.Revision }}"
	}

	cb := NewConfigBuilder().
		SetSettings(&settings).
		SetAPIVersion("batch/v1").
		SetKind("Job").
		SetName(name).
		AddModifier(helm.Comment(instanceGroup.GetLongDescription()))
	job, err := cb.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build a new kube config: %v", err)
	}
	job.Add("spec", helm.NewMapping("template", podTemplate))
	addFeatureCheck(instanceGroup, job)

	return job.Sort(), nil
}
