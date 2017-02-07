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
	if len(role.Run.ExposedPorts) == 0 {
		// Kubernetes refuses to create services with no ports, so we should
		// not return anything at all in this case
		return nil
	}

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
			Ports: make([]apiv1.ServicePort, 0, len(role.Run.ExposedPorts)),
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
