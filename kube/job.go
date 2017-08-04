package kube

import (
	"fmt"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"
	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	extra "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// NewJob creates a new Job for the given role, as well as any objects it depends on
func NewJob(role *model.Role, settings *ExportSettings, verbosity util.Verbosity) (*extra.Job, error) {
	podTemplate, err := NewPodTemplate(role, settings, verbosity)
	if err != nil {
		return nil, err
	}

	if role.Run == nil {
		return nil, fmt.Errorf("Role %s has no run information", role.Name)
	}

	// Jobs must have a restart policy that isn't "always"
	switch role.Run.FlightStage {
	case model.FlightStageManual:
		podTemplate.Spec.RestartPolicy = apiv1.RestartPolicyNever
	case model.FlightStageFlight, model.FlightStagePreFlight, model.FlightStagePostFlight:
		podTemplate.Spec.RestartPolicy = apiv1.RestartPolicyOnFailure
	default:
		return nil, fmt.Errorf("Role %s has unexpected flight stage %s", role.Name, role.Run.FlightStage)
	}

	return &extra.Job{
		TypeMeta: meta.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Job",
		},
		ObjectMeta: apiv1.ObjectMeta{
			Name: role.Name,
		},
		Spec: extra.JobSpec{
			Template: podTemplate,
		},
	}, nil
}
