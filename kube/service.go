package kube

import (
	"fmt"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
)

// NewServiceList creates a list of services
// clustering should be true if a kubernetes headless service should be created
// (for self-clustering roles, to reach each pod individually)
func NewServiceList(role *model.InstanceGroup, clustering bool, settings ExportSettings) (helm.Node, error) {
	var items []helm.Node

	if clustering {
		svc, err := newClusteringService(role, settings)
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

// createPorts generates a helm mapping according to the JobExposedPort
func createPorts(settings ExportSettings, serviceType newServiceType, roleName string, port model.JobExposedPort) []helm.Node {
	var ports []helm.Node
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
			ports = append(ports, newPort)
		}
	}

	return ports
}

// newClusteringService creates a new k8s service for the overall instance group.
// This allows individual pods to be addressed by their index.
func newClusteringService(role *model.InstanceGroup, settings ExportSettings) (helm.Node, error) {
	var ports []helm.Node
	for _, job := range role.JobReferences {
		for _, port := range job.ContainerProperties.BoshContainerization.Ports {
			ports = append(ports, createPorts(settings, newServiceTypeHeadless, role.Name, port)...)
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
	if settings.IstioComplied && role.HasTag(model.RoleTagIstioManaged) {
		selector.Add(AppNameLabel, role.Name)
	}
	spec.Add("selector", selector)

	spec.Add("clusterIP", "None")
	spec.Add("ports", helm.NewNode(ports))

	service := newKubeConfig(settings, "v1", "Service", role.Name+"-set")
	service.Add("spec", spec.Sort())

	return service, nil
}

// newService creates a new k8s service (ClusterIP or LoadBalanced) for a job
func newService(role *model.InstanceGroup, job *model.JobReference, serviceType newServiceType, settings ExportSettings) (helm.Node, error) {
	var ports []helm.Node

	for _, port := range job.ContainerProperties.BoshContainerization.Ports {
		if serviceType == newServiceTypePublic && !port.Public {
			// Skip non-public ports when creating public services
			continue
		}

		ports = append(ports, createPorts(settings, serviceType, role.Name, port)...)
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

	if settings.IstioComplied && role.HasTag(model.RoleTagIstioManaged) {
		selector.Add(AppNameLabel, role.Name)
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

	serviceName := job.ContainerProperties.BoshContainerization.ServiceName
	if len(serviceName) == 0 {
		serviceName = util.ConvertNameToKey(role.Name + "-" + job.Name)
	}

	switch serviceType {
	case newServiceTypeHeadless:
		serviceName += "-set"
	case newServiceTypePrivate:
		// all set
	case newServiceTypePublic:
		serviceName += "-public"
	default:
		panic(fmt.Sprintf("Unexpected service type %d", serviceType))
	}

	service := newKubeConfig(settings, "v1", "Service", serviceName)
	service.Add("spec", spec.Sort())

	return service, nil
}
