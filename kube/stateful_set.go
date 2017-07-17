package kube

import (
	"fmt"

	"github.com/SUSE/fissile/model"
	"k8s.io/client-go/pkg/api/resource"
	meta "k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	v1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
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

	volumeClaimTemplates := getVolumeClaims(role, settings.CreateHelmChart)

	svcList, err := NewClusterIPServiceList(role, true, settings)
	if err != nil {
		return nil, nil, err
	}

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
			ServiceName:          fmt.Sprintf("%s-set", role.Name),
			Template:             podTemplate,
			VolumeClaimTemplates: volumeClaimTemplates,
		},
	}, svcList, nil
}

// getVolumeClaims returns the list of persistent volume claims from a role
func getVolumeClaims(role *model.Role, createHealmChart bool) []v1.PersistentVolumeClaim {
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

	if createHealmChart {
		for i := range types {
			types[i].storageClass = fmt.Sprintf("((k8s.storage-class.%s))", types[i].storageClass)
		}
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
