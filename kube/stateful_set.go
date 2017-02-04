package kube

import (
	"fmt"

	"github.com/hpcloud/fissile/model"
	apiv1 "k8s.io/client-go/1.5/pkg/api/v1"
	appsv1alpha1 "k8s.io/client-go/1.5/pkg/apis/apps/v1alpha1"
	"k8s.io/client-go/1.5/pkg/runtime"
)

// NewStatefulSet returns a k8s stateful set for the given role
func NewStatefulSet(role *model.Role) (*appsv1alpha1.PetSet, *apiv1.List, error) {
	// For each StatefulSet, we need two services -- one for the public (inside
	// the namespace) endpoint, and one headless service to control the pods.
	if role == nil {
		panic(fmt.Sprintf("No role given"))
	}

	podTemplate, templateDeps, err := NewPodTemplate(role)

	if err != nil {
		return nil, nil, err
	}

	var volumeClaimTemplates []apiv1.PersistentVolumeClaim
	for _, templateDep := range templateDeps {
		if volumeClaim, ok := templateDep.(*apiv1.PersistentVolumeClaim); ok {
			volumeClaimTemplates = append(volumeClaimTemplates, *volumeClaim)
		}
	}

	return &appsv1alpha1.PetSet{
			ObjectMeta: apiv1.ObjectMeta{
				Name: role.Name,
				Labels: map[string]string{
					RoleNameLabel: role.Name,
				},
			},
			Spec: appsv1alpha1.PetSetSpec{
				Replicas:             &role.Run.Scaling.Min,
				ServiceName:          fmt.Sprintf("%s-pod", role.Name),
				Template:             podTemplate,
				VolumeClaimTemplates: volumeClaimTemplates,
			},
		}, &apiv1.List{
			Items: []runtime.RawExtension{
				runtime.RawExtension{
					Object: NewClusterIPService(role, false),
				},
				runtime.RawExtension{
					Object: NewClusterIPService(role, true),
				},
			},
		}, nil
}