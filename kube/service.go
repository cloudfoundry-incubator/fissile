package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// NewServiceList creates a list of services
// clustering should be true if a kubernetes headless service should be created
// (for self-clustering roles, to reach each pod individually)
func NewServiceList(role *model.InstanceGroup, clustering bool, settings ExportSettings) (helm.Node, error) {
	var items []helm.Node

	if clustering {
		// Create headless, private service
		svc, err := newService(role, newServiceTypeHeadless, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			items = append(items, svc)
		}
	}

	if !role.HasTag(model.RoleTagHeadless) {
		// Create private service
		svc, err := newService(role, newServiceTypePrivate, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			items = append(items, svc)
		}
	}

	// Create public service
	svc, err := newService(role, newServiceTypePublic, settings)
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

// newServiceType is the type of the service to create
type newServiceType int

const (
	_                      = iota
	newServiceTypeHeadless // Create a headless service (for clustering)
	newServiceTypePrivate  // Create a private endpoint service (internal traffic)
	newServiceTypePublic   // Create a public endpoint service (externally visible traffic)
)

// newService creates a new k8s service (ClusterIP or LoadBalanced)
func newService(role *model.InstanceGroup, serviceType newServiceType, settings ExportSettings) (helm.Node, error) {
	var ports []helm.Node
	for _, port := range role.Run.ExposedPorts {
		if serviceType == newServiceTypePublic && !port.Public {
			// Skip non-public ports when creating public services
			continue
		}

		if settings.CreateHelmChart && port.CountIsConfigurable {
			sizing := fmt.Sprintf(".Values.sizing.%s.ports.%s", makeVarName(role.Name), makeVarName(port.Name))

			block := fmt.Sprintf("range $port := until (int %s.count)", sizing)

			portName := port.Name
			if port.Max > 1 {
				portName = fmt.Sprintf("%s-{{ $port }}", portName)
			}

			var portNumber string
			if port.PortIsConfigurable {
				portNumber = fmt.Sprintf("{{ add (int $%s.port) $port }}", sizing)
			} else {
				portNumber = fmt.Sprintf("{{ add %d $port }}", port.ExternalPort)
			}

			newPort := helm.NewMapping(
				"name", portName,
				"port", portNumber,
				"protocol", port.Protocol,
			)
			newPort.Set(helm.Block(block))
			if serviceType == newServiceTypeHeadless {
				newPort.Add("targetPort", 0)
			} else {
				newPort.Add("targetPort", portName)
			}
			ports = append(ports, newPort)
		} else {
			for portIndex := 0; portIndex < port.Count; portIndex++ {
				portName := port.Name
				if port.Max > 1 {
					portName = fmt.Sprintf("%s-%d", portName, portIndex)
				}

				var portNumber interface{}
				if settings.CreateHelmChart && port.PortIsConfigurable {
					portNumber = fmt.Sprintf("{{ add (int $.Values.sizing.%s.ports.%s.port) %d }}",
						makeVarName(role.Name), makeVarName(port.Name), portIndex)
				} else {
					portNumber = port.ExternalPort + portIndex
				}

				newPort := helm.NewMapping(
					"name", portName,
					"port", portNumber,
					"protocol", port.Protocol,
				)

				if serviceType == newServiceTypeHeadless {
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

	selector := helm.NewMapping(RoleNameLabel, role.Name)
	if role.HasTag(model.RoleTagActivePassive) {
		selector.Add("skiff-role-active", "true")
	}
	spec.Add("selector", selector)

	if serviceType == newServiceTypeHeadless {
		spec.Add("clusterIP", "None")
	}
	if serviceType == newServiceTypePublic {
		if settings.CreateHelmChart {
			spec.Add("externalIPs", "{{ .Values.kube.external_ips | toJson }}", helm.Block("if not .Values.services.loadbalanced"))
			spec.Add("type", "LoadBalancer", helm.Block("if .Values.services.loadbalanced"))
		} else {
			spec.Add("externalIPs", []string{"192.168.77.77"})
		}
	}
	spec.Add("ports", helm.NewNode(ports))

	var serviceName string
	switch serviceType {
	case newServiceTypeHeadless:
		serviceName = role.Name + "-set"
	case newServiceTypePrivate:
		serviceName = role.Name
	case newServiceTypePublic:
		serviceName = role.Name + "-public"
	default:
		panic(fmt.Sprintf("Unexpected service type %d", serviceType))
	}
	service := newTypeMeta("v1", "Service")
	service.Add("metadata", helm.NewMapping("name", serviceName))
	service.Add("spec", spec.Sort())

	return service, nil
}
