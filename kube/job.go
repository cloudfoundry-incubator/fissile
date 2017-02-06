package kube

import (
	"github.com/hpcloud/fissile/model"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	batchv2alpha1 "k8s.io/client-go/pkg/apis/batch/v2alpha1"
	"k8s.io/client-go/pkg/runtime"
)

// NewJob creates a new Job for the given role, as well as any objects it depends on
func NewJob(role *model.Role) (*batchv2alpha1.Job, []runtime.Object, error) {
	podTemplate, deps, err := NewPodTemplate(role)
	if err != nil {
		return nil, nil, err
	}
	// Jobs must have a restart policy that isn't "always"
	podTemplate.Spec.RestartPolicy = apiv1.RestartPolicyOnFailure
	return &batchv2alpha1.Job{
		ObjectMeta: apiv1.ObjectMeta{
			Name: role.Name,
		},
		Spec: batchv2alpha1.JobSpec{
			Template: podTemplate,
		},
	}, deps, nil
}
