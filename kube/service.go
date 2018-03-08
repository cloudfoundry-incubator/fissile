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
	for _, portDef := range role.Run.ExposedPorts {
		if public && !portDef.Public {
			continue
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
			port := helm.NewMapping(
				"name", portInfoEntry.name,
				"port", portInfoEntry.port,
				"protocol", strings.ToUpper(portDef.Protocol),
			)
			if headless {
				port.Add("targetPort", 0)
			} else {
				port.Add("targetPort", portInfoEntry.name)
			}
			ports = append(ports, port)
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
			externalIP = strings.Replace(`{{
				default
					( list .Values.kube.external_ip )
					.Values.kube.external_ips
				| toJson
			}}`, "\n", " ", -1)
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
