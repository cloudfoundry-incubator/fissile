package kube

import (
	"fmt"
	"strings"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// NewServiceList creates a list of services
// clustering should be true if a kubernetes headless service should be created
// (for self-clustering roles, to reach each pod individually)
func NewServiceList(role *model.InstanceGroup, clustering bool, settings ExportSettings) (helm.Node, error) {
	var items []helm.Node

	if clustering {
		svc, err := newGenericService(role, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			items = append(items, svc)
		}
	}

	for _, job := range role.JobReferences {
		if clustering {
			// Create headless, private service
			svc, err := newService(role, job, newServiceTypeHeadless, settings)
			if err != nil {
				return nil, err
			}
			if svc != nil {
				items = append(items, svc)
			}
		}

		// Create private service
		svc, err := newService(role, job, newServiceTypePrivate, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			items = append(items, svc)
		}

		// Create public service
		svc, err = newService(role, job, newServiceTypePublic, settings)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			items = append(items, svc)
		}
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

// createPort generates a helm mapping according to the JobExposedPort
func createPort(settings ExportSettings, serviceType newServiceType, roleName string, port model.JobExposedPort, ports *[]helm.Node) {
	if settings.CreateHelmChart && port.CountIsConfigurable {
		sizing := fmt.Sprintf(".Values.sizing.%s.ports.%s", makeVarName(roleName), makeVarName(port.Name))

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
		*ports = append(*ports, newPort)
	} else {
		for portIndex := 0; portIndex < port.Count; portIndex++ {
			portName := port.Name
			if port.Max > 1 {
				portName = fmt.Sprintf("%s-%d", portName, portIndex)
			}

			var portNumber interface{}
			if settings.CreateHelmChart && port.PortIsConfigurable {
				portNumber = fmt.Sprintf("{{ add (int $.Values.sizing.%s.ports.%s.port) %d }}",
					makeVarName(roleName), makeVarName(port.Name), portIndex)
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
				// Use number instead of name here, in case we have multiple
				// port definitions with the same internal port
				newPort.Add("targetPort", port.InternalPort+portIndex)
			}
			*ports = append(*ports, newPort)
		}
	}
}

// newGenericService creates a new k8s service (ClusterIP or LoadBalanced)
func newGenericService(role *model.InstanceGroup, settings ExportSettings) (helm.Node, error) {
	var ports []helm.Node
	for _, job := range role.JobReferences {
		for _, port := range job.ContainerProperties.BoshContainerization.Ports {
			createPort(settings, newServiceTypeHeadless, role.Name, port, &ports)
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

	spec.Add("clusterIP", "None")
	spec.Add("ports", helm.NewNode(ports))

	serviceName := role.Name + "-set"
	service := newTypeMeta("v1", "Service")
	service.Add("metadata", helm.NewMapping("name", serviceName))
	service.Add("spec", spec.Sort())

	return service, nil
}

// newService creates a new k8s service (ClusterIP or LoadBalanced)
func newService(role *model.InstanceGroup, job *model.JobReference, serviceType newServiceType, settings ExportSettings) (helm.Node, error) {
	var ports []helm.Node

	for _, port := range job.ContainerProperties.BoshContainerization.Ports {
		if serviceType == newServiceTypePublic && !port.Public {
			// Skip non-public ports when creating public services
			continue
		}

		createPort(settings, serviceType, role.Name, port, &ports)
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

	jobName := strings.Replace(job.Name, "_", "-", -1)

	var serviceName string
	switch serviceType {
	case newServiceTypeHeadless:
		serviceName = role.Name + "-" + jobName + "-set"
	case newServiceTypePrivate:
		serviceName = role.Name + "-" + jobName
	case newServiceTypePublic:
		serviceName = role.Name + "-" + jobName + "-public"
	default:
		panic(fmt.Sprintf("Unexpected service type %d", serviceType))
	}
	service := newTypeMeta("v1", "Service")
	service.Add("metadata", helm.NewMapping("name", serviceName))
	service.Add("spec", spec.Sort())

	return service, nil
}
