package kube

import (
	"github.com/hpcloud/fissile/model"
	"k8s.io/client-go/pkg/api/v1"
	extra "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/runtime"
)

// NewJob creates a new Job for the given role, as well as any objects it depends on
func NewJob(role *model.Role, settings *KubeExportSettings) (*extra.Job, []runtime.Object, error) {
	podTemplate, deps, err := NewPodTemplate(role, settings)
	if err != nil {
		return nil, nil, err
	}
	// Jobs must have a restart policy that isn't "always"
	podTemplate.Spec.RestartPolicy = v1.RestartPolicyOnFailure
	return &extra.Job{
		ObjectMeta: v1.ObjectMeta{
			Name: role.Name,
		},
		Spec: extra.JobSpec{
			Template: podTemplate,
		},
	}, deps, nil
}
