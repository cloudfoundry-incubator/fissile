package kube

import (
	"fmt"

	"github.com/hpcloud/fissile/model"
	"k8s.io/client-go/pkg/api/resource"
	meta "k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	v1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/client-go/pkg/runtime"
)

// NewStatefulSet returns a k8s stateful set for the given role
func NewStatefulSet(role *model.Role, settings *ExportSettings) (*v1beta1.StatefulSet, *v1.List, error) {
	// For each StatefulSet, we need two services -- one for the public (inside
	// the namespace) endpoint, and one headless service to control the pods.
	if role == nil {
		panic(fmt.Sprintf("No role given"))
	}

	podTemplate, err := NewPodTemplate(role, settings)

	if err != nil {
		return nil, nil, err
	}

	volumeClaimTemplates := getVolumeClaims(role)

	return &v1beta1.StatefulSet{
			TypeMeta: meta.TypeMeta{
				APIVersion: "apps/v1beta1",
				Kind:       "StatefulSet",
			},
			ObjectMeta: v1.ObjectMeta{
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
		}, &v1.List{
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

// getVolumeClaims returns the list of persistent volume claims from a role
func getVolumeClaims(role *model.Role) []v1.PersistentVolumeClaim {
	totalLength := len(role.Run.PersistentVolumes) + len(role.Run.SharedVolumes)
	claims := make([]v1.PersistentVolumeClaim, 0, totalLength)

	types := []struct {
		volumeDefinitions []*model.RoleRunVolume
		storageClass      string
		accessMode        v1.PersistentVolumeAccessMode
	}{
		{
			role.Run.PersistentVolumes,
			"persistent",
			v1.ReadWriteOnce,
		},
		{
			role.Run.SharedVolumes,
			"shared",
			v1.ReadWriteMany,
		},
	}

	for _, volumeTypeInfo := range types {
		for _, volume := range volumeTypeInfo.volumeDefinitions {
			pvc := v1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name: volume.Tag,
					Annotations: map[string]string{
						"volume.beta.kubernetes.io/storage-class": volumeTypeInfo.storageClass,
					},
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						volumeTypeInfo.accessMode,
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: *resource.NewScaledQuantity(int64(volume.Size), resource.Giga),
						},
					},
				},
			}

			claims = append(claims, pvc)
		}
	}

	return claims
}
