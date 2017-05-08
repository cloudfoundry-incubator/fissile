package kube

import (
	"fmt"
	"strings"

	"github.com/SUSE/fissile/model"
	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
)

// NewClusterIPService creates a new k8s ClusterIP service
func NewClusterIPService(role *model.Role, headless bool) (*apiv1.Service, error) {
	if len(role.Run.ExposedPorts) == 0 {
		// Kubernetes refuses to create services with no ports, so we should
		// not return anything at all in this case
		return nil, nil
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
		protocol := apiv1.ProtocolTCP
		switch strings.ToLower(portDef.Protocol) {
		case "tcp":
			protocol = apiv1.ProtocolTCP
		case "udp":
			protocol = apiv1.ProtocolUDP
		}

		minPort, maxPort, err := parsePortRange(portDef.External, portDef.Name, "external")
		if err != nil {
			return nil, err
		}
		portInfos, err := getPortInfo(portDef.Name, minPort, maxPort)
		if err != nil {
			return nil, err
		}

		for _, portInfoEntry := range portInfos {
			svcPort := apiv1.ServicePort{
				Name:     portInfoEntry.name,
				Port:     portInfoEntry.port,
				Protocol: protocol,
			}
			if !headless {
				svcPort.TargetPort = intstr.FromString(portInfoEntry.name)
			}
			service.Spec.Ports = append(service.Spec.Ports, svcPort)
		}
		if portDef.Public {
			service.Spec.ExternalIPs = []string{"192.168.77.77"} // TODO Make this work on not-vagrant
		}
	}
	return service, nil
}
