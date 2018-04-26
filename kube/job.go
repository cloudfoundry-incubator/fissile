package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"
)

// NewJob creates a new Job for the given role, as well as any objects it depends on
func NewJob(role *model.Role, settings ExportSettings, grapher util.ModelGrapher) (helm.Node, error) {
	podTemplate, err := NewPodTemplate(role, settings, grapher)
	if err != nil {
		return nil, err
	}

	// Jobs must have a restart policy that isn't "always"
	switch role.Run.FlightStage {
	case model.FlightStageManual:
		podTemplate.Get("spec", "restartPolicy").SetValue("Never")
	case model.FlightStageFlight, model.FlightStagePreFlight, model.FlightStagePostFlight:
		podTemplate.Get("spec", "restartPolicy").SetValue("OnFailure")
	default:
		return nil, fmt.Errorf("Role %s has unexpected flight stage %s", role.Name, role.Run.FlightStage)
	}

	name := role.Name
	apiVersion := "batch/v1"
	if settings.CreateHelmChart {
		name += "-{{ .Release.Revision }}"
	}

	metadata := helm.NewMapping()
	metadata.Add("name", name)
	if role.Run.ObjectAnnotations != nil {
		metadata.Add("annotations", *role.Run.ObjectAnnotations)
	}
	metadata.Sort()

	job := newTypeMeta(apiVersion, "Job", helm.Comment(role.GetLongDescription()))
	job.Add("metadata", metadata)
	job.Add("spec", helm.NewMapping("template", podTemplate))

	return job.Sort(), nil
}
