package kube

import (
	"fmt"

	"github.com/hpcloud/fissile/model"
	apiv1 "k8s.io/client-go/1.5/pkg/api/v1"
	appsv1alpha1 "k8s.io/client-go/1.5/pkg/apis/apps/v1alpha1"
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/util/intstr"
)

// NewStatefulSet returns a k8s stateful set for the given role
func NewStatefulSet(role *model.Role) (*appsv1alpha1.PetSet, *apiv1.List) {
	// For each StatefulSet, we need two services -- one for the public (inside
	// the namespace) endpoint, and one headless service to control the pods.
	if role == nil {
		panic(fmt.Sprintf("No role given"))
	}
	endpointService := &apiv1.Service{
		ObjectMeta: apiv1.ObjectMeta{
			Name: role.Name,
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Selector: map[string]string{
				RoleNameLabel: role.Name,
			},
		},
	}
	headlessService := &apiv1.Service{
		ObjectMeta: apiv1.ObjectMeta{
			Name: fmt.Sprintf("%s-pod", role.Name),
		},
		Spec: apiv1.ServiceSpec{
			Type:      apiv1.ServiceTypeClusterIP,
			ClusterIP: apiv1.ClusterIPNone,
			Selector: map[string]string{
				RoleNameLabel: role.Name,
			},
		},
	}

	podTemplate := NewPodTemplate(role)

	for _, container := range podTemplate.Spec.Containers {
		for _, port := range container.Ports {
			for _, svc := range []*apiv1.Service{endpointService, headlessService} {
				svcPort := apiv1.ServicePort{
					Name: port.Name,
				}
				if port.HostPort != 0 && port.HostPort != port.ContainerPort {
					svcPort.Port = port.HostPort
				} else {
					svcPort.Port = port.ContainerPort
				}
				if svc != headlessService {
					svcPort.TargetPort = intstr.FromString(port.Name)
				}
				svc.Spec.Ports = append(svc.Spec.Ports, svcPort)
			}
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
				Replicas:    &role.Run.Scaling.Min,
				ServiceName: fmt.Sprintf("%s-pod", role.Name),
				Template:    podTemplate,
			},
		}, &apiv1.List{
			Items: []runtime.RawExtension{
				runtime.RawExtension{
					Object: endpointService,
				},
				runtime.RawExtension{
					Object: headlessService,
				},
			},
		}
}
