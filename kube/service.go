package kube

import (
	"fmt"
	"strings"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// NewClusterIPServiceList creates a list of ClusterIP services
func NewClusterIPServiceList(role *model.Role, headless, private bool, settings ExportSettings) (helm.Node, error) {
	var items []helm.Node

	if headless {
		// Create headless, private service
		svc, err := NewClusterIPService(role, true, false, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			items = append(items, svc)
		}
	}

	if private {
		// Create private service
		svc, err := NewClusterIPService(role, false, false, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			items = append(items, svc)
		}
	}

	// Create public service
	svc, err := NewClusterIPService(role, false, true, settings)
	if err != nil {
		return nil, err
	}
	if svc != nil {
		items = append(items, svc)
	}
	if len(items) == 0 {
		return nil, nil
	}

	list := newTypeMeta("v1", "List")
	list.Add("items", helm.NewNode(items))

	return list.Sort(), nil
}

// NewClusterIPService creates a new k8s ClusterIP service
func NewClusterIPService(role *model.Role, headless bool, public bool, settings ExportSettings) (helm.Node, error) {
	var ports []helm.Node
	for _, port := range role.Run.ExposedPorts {
		if public && !port.Public {
			continue
		}
		if settings.CreateHelmChart && port.UserConfigurable {
			sizing := fmt.Sprintf(".Values.sizing.%s.ports.%s", makeVarName(role.Name), makeVarName(port.Name))

			block := fmt.Sprintf("range $port := untilStep (int %s.port) (int (add %s.port %s.count)) 1", sizing, sizing, sizing)
			newPort := helm.NewMapping()
			newPort.Set(helm.Block(block))

			portName := port.Name
			if port.Max > 1 {
				portName = fmt.Sprintf("%s-{{ $port }}", portName)
			}
			newPort.Add("name", portName)
			newPort.Add("port", "{{ $port }}")
			newPort.Add("protocol", port.Protocol)
			if headless {
				newPort.Add("targetPort", 0)
			} else {
				newPort.Add("targetPort", portName)
			}
			ports = append(ports, newPort)
		} else {
			for portNumber := port.ExternalPort; portNumber < port.ExternalPort+port.Count; portNumber++ {
				portName := port.Name
				if port.Max > 1 {
					portName = fmt.Sprintf("%s-%d", portName, portNumber)
				}
				newPort := helm.NewMapping(
					"name", portName,
					"port", portNumber,
					"protocol", port.Protocol,
				)

				if headless {
					newPort.Add("targetPort", 0)
				} else {
					newPort.Add("targetPort", portName)
				}
				ports = append(ports, newPort)
			}
		}
	}
	if len(ports) == 0 {
		// Kubernetes refuses to create services with no ports, so we should
		// not return anything at all in this case
		return nil, nil
	}

	spec := helm.NewMapping()
	spec.Add("selector", helm.NewMapping(RoleNameLabel, role.Name))

	if settings.CreateHelmChart {
		spec.Add("type", "{{ if .Values.services.loadbalanced }} LoadBalancer {{ else }} ClusterIP {{ end }}")
	} else {
		spec.Add("type", "ClusterIP")
	}
	if headless {
		if settings.CreateHelmChart {
			spec.Add("clusterIP", "None", helm.Block("if not .Values.services.loadbalanced"))
		} else {
			spec.Add("clusterIP", "None")
		}
	}
	if public {
		externalIP := "[ 192.168.77.77 ]"
		if settings.CreateHelmChart {
			// Backwards compatibility: If .kube.external_ips doesn't exist,
			// use .kube.external_ip instead (as the single address)
			externalIP = strings.Join(strings.Fields(`{{
				default
					( append .Values.kube.external_ips
						( .Values.kube.external_ip )
					)
					.Values.kube.external_ips
				| toJson
			}}`), " ")
		}
		spec.Add("externalIPs", externalIP)
	}
	spec.Add("ports", helm.NewNode(ports))

	serviceName := role.Name
	if headless {
		serviceName = fmt.Sprintf("%s-set", role.Name)
	} else if public {
		serviceName = fmt.Sprintf("%s-public", role.Name)
	}

	service := newTypeMeta("v1", "Service")
	service.Add("metadata", helm.NewMapping("name", serviceName))
	service.Add("spec", spec.Sort())

	return service, nil
}
