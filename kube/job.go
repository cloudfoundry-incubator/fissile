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
	apiVersion := "batch/v1"
	if settings.CreateHelmChart {
		name += "-{{ .Release.Revision }}"
	}

	metadata := helm.NewMapping()
	metadata.Add("name", name)
	metadata.Sort()

	job := newTypeMeta(apiVersion, "Job", helm.Comment(instanceGroup.GetLongDescription()))
	job.Add("metadata", metadata)
	job.Add("spec", helm.NewMapping("template", podTemplate))

	return job.Sort(), nil
}
