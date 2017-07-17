package kube

import (
	"fmt"
	"strings"

	"github.com/SUSE/fissile/model"
	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/util/intstr"
)

// NewClusterIPServiceList creates a list of ClusterIP services
func NewClusterIPServiceList(role *model.Role, headless bool, settings *ExportSettings) (*apiv1.List, error) {
	list := &apiv1.List{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "List",
		},
		Items: []runtime.RawExtension{},
	}
	if headless {
		// Create headless, private service
		svc, err := NewClusterIPService(role, true, false, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			list.Items = append(list.Items, runtime.RawExtension{Object: svc})
		}
	}
	// Create private service
	svc, err := NewClusterIPService(role, false, false, settings)
	if err != nil {
		return nil, err
	}
	if svc != nil {
		list.Items = append(list.Items, runtime.RawExtension{Object: svc})
	}
	// Create public service
	svc, err = NewClusterIPService(role, false, true, settings)
	if err != nil {
		return nil, err
	}
	if svc != nil {
		list.Items = append(list.Items, runtime.RawExtension{Object: svc})
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return list, nil
}

// NewClusterIPService creates a new k8s ClusterIP service
func NewClusterIPService(role *model.Role, headless bool, public bool, settings *ExportSettings) (*apiv1.Service, error) {
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
		service.ObjectMeta.Name = fmt.Sprintf("%s-set", role.Name)
		service.Spec.ClusterIP = apiv1.ClusterIPNone
	} else if public {
		service.ObjectMeta.Name = fmt.Sprintf("%s-public", role.Name)
	}
	for _, portDef := range role.Run.ExposedPorts {
		if public && !portDef.Public {
			continue
		}
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
		if public {
			if settings.CreateHelmChart {
				service.Spec.ExternalIPs = []string{"{{ .Values.kube.external_ip | quote }}"}
			} else {
				service.Spec.ExternalIPs = []string{"192.168.77.77"}
			}
		}
	}
	if len(service.Spec.Ports) == 0 {
		// Kubernetes refuses to create services with no ports, so we should
		// not return anything at all in this case
		return nil, nil
	}
	return service, nil
}
