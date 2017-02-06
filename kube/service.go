package kube

import (
	"fmt"

	"github.com/hpcloud/fissile/model"
	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
)

// NewClusterIPService creates a new k8s ClusterIP service
func NewClusterIPService(role *model.Role, headless bool) *apiv1.Service {
	service := &apiv1.Service{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
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
	if headless {
		service.ObjectMeta.Name = fmt.Sprintf("%s-pod", role.Name)
		service.Spec.ClusterIP = apiv1.ClusterIPNone
	}
	for _, portDef := range role.Run.ExposedPorts {
		svcPort := apiv1.ServicePort{
			Name: portDef.Name,
			Port: portDef.External,
		}
		if !headless {
			svcPort.TargetPort = intstr.FromInt(int(portDef.Internal))
		}
		service.Spec.Ports = append(service.Spec.Ports, svcPort)
	}
	return service
}
