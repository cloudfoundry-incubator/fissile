package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// NewJob creates a new Job for the given role, as well as any objects it depends on
func NewJob(role *model.Role, settings *ExportSettings) (helm.Node, error) {
	podTemplate, err := NewPodTemplate(role, settings)
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

	apiVersion := "extensions/v1beta1"
	if settings.CreateHelmChart {
		// Job objects become a regular feature in kube 1.6
		apiVersion = fmt.Sprintf("{{ if %s -}} batch/v1 {{- else -}} %s {{- end }}", minKubeVersion(1, 6), apiVersion)
	}
	job := newTypeMeta(apiVersion, "Job")
	job.Add("metadata", helm.NewMapping("name", role.Name))
	job.Add("spec", helm.NewMapping("template", podTemplate))

	return job.Sort(), nil
}
