package kube

import (
	"fmt"

	"github.com/hpcloud/fissile/model"
	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	v1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/client-go/pkg/runtime"
)

// NewStatefulSet returns a k8s stateful set for the given role
func NewStatefulSet(role *model.Role, repository string, defaults map[string]string) (*v1beta1.StatefulSet, *apiv1.List, error) {
	// For each StatefulSet, we need two services -- one for the public (inside
	// the namespace) endpoint, and one headless service to control the pods.
	if role == nil {
		panic(fmt.Sprintf("No role given"))
	}

	podTemplate, templateDeps, err := NewPodTemplate(role, repository, defaults)

	if err != nil {
		return nil, nil, err
	}

	var volumeClaimTemplates []apiv1.PersistentVolumeClaim
	for _, templateDep := range templateDeps {
		if volumeClaim, ok := templateDep.(*apiv1.PersistentVolumeClaim); ok {
			volumeClaimTemplates = append(volumeClaimTemplates, *volumeClaim)
		}
	}

	return &v1beta1.StatefulSet{
			TypeMeta: meta.TypeMeta{
				APIVersion: "apps/v1beta1",
				Kind:       "StatefulSet",
			},
			ObjectMeta: apiv1.ObjectMeta{
				Name: role.Name,
				Labels: map[string]string{
					RoleNameLabel: role.Name,
				},
			},
			Spec: v1beta1.StatefulSetSpec{
				Replicas:             &role.Run.Scaling.Min,
				ServiceName:          fmt.Sprintf("%s-pod", role.Name),
				Template:             podTemplate,
				VolumeClaimTemplates: volumeClaimTemplates,
			},
		}, &apiv1.List{
			TypeMeta: meta.TypeMeta{
				APIVersion: "v1",
				Kind:       "List",
			},
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
