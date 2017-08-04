package kube

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/SUSE/fissile/builder"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"

	"k8s.io/client-go/pkg/api/resource"
	meta "k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
)

// monitPort is the port monit runs on in the pods
const monitPort = 2289

// NewPodTemplate creates a new pod template spec for a given role, as well as
// any objects it depends on
func NewPodTemplate(role *model.Role, settings *ExportSettings, verbosity util.Verbosity) (v1.PodTemplateSpec, error) {

	vars, err := getEnvVars(role, settings.Defaults, settings.Secrets, settings.CreateHelmChart)
	if err != nil {
		return v1.PodTemplateSpec{}, err
	}

	var resources v1.ResourceRequirements

	if settings.UseMemoryLimits {
		resources = v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", role.Run.Memory)),
			},
		}
	}

	securityContext := getSecurityContext(role)

	ports, err := getContainerPorts(role)
	if err != nil {
		return v1.PodTemplateSpec{}, err
	}

	image, err := getContainerImageName(role, settings, verbosity)
	if err != nil {
		return v1.PodTemplateSpec{}, err
	}

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Name: role.Name,
			Labels: map[string]string{
				RoleNameLabel: role.Name,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				v1.Container{
					Name:            role.Name,
					Image:           image,
					Ports:           ports,
					VolumeMounts:    getVolumeMounts(role),
					Env:             vars,
					Resources:       resources,
					SecurityContext: securityContext,
				},
			},
			RestartPolicy: v1.RestartPolicyAlways,
			DNSPolicy:     v1.DNSClusterFirst,
		},
	}

	livenessProbe, err := getContainerLivenessProbe(role)
	if err != nil {
		return v1.PodTemplateSpec{}, err
	}
	readinessProbe, err := getContainerReadinessProbe(role)
	if err != nil {
		return v1.PodTemplateSpec{}, err
	}
	if livenessProbe != nil || readinessProbe != nil {
		for i := range podSpec.Spec.Containers {
			podSpec.Spec.Containers[i].LivenessProbe = livenessProbe
			podSpec.Spec.Containers[i].ReadinessProbe = readinessProbe
		}
	}

	return podSpec, nil
}

// NewPod creates a new Pod for the given role, as well as any objects it depends on
func NewPod(role *model.Role, settings *ExportSettings, verbosity util.Verbosity) (*v1.Pod, error) {
	podTemplate, err := NewPodTemplate(role, settings, verbosity)
	if err != nil {
		return nil, err
	}

	if role.Run == nil {
		return nil, fmt.Errorf("Role %s has no run information", role.Name)
	}

	// Pod must have a restart policy that isn't "always"
	switch role.Run.FlightStage {
	case model.FlightStageManual:
		podTemplate.Spec.RestartPolicy = v1.RestartPolicyNever
	case model.FlightStageFlight, model.FlightStagePreFlight, model.FlightStagePostFlight:
		podTemplate.Spec.RestartPolicy = v1.RestartPolicyOnFailure
	default:
		return nil, fmt.Errorf("Role %s has unexpected flight stage %s", role.Name, role.Run.FlightStage)
	}

	return &v1.Pod{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: role.Name,
			Labels: map[string]string{
				RoleNameLabel: role.Name,
			},
		},
		Spec: podTemplate.Spec,
	}, nil
}

// getContainerImageName returns the name of the docker image to use for a role
func getContainerImageName(role *model.Role, settings *ExportSettings, verbosity util.Verbosity) (string, error) {
	devVersion, err := role.GetRoleDevVersion(settings.Opinions, settings.FissileVersion, verbosity)
	if err != nil {
		return "", err
	}

	imageName := builder.GetRoleDevImageName(settings.Registry, settings.Organization, settings.Repository, role, devVersion)

	return imageName, nil
}

// getContainerPorts returns a list of ports for a role
func getContainerPorts(role *model.Role) ([]v1.ContainerPort, error) {
	result := make([]v1.ContainerPort, 0, len(role.Run.ExposedPorts))

	for _, port := range role.Run.ExposedPorts {
		var protocol v1.Protocol

		switch strings.ToLower(port.Protocol) {
		case "tcp":
			protocol = v1.ProtocolTCP
		case "udp":
			protocol = v1.ProtocolUDP
		}

		// Convert port range specifications to port numbers
		minInternalPort, maxInternalPort, err := parsePortRange(port.Internal, port.Name, "internal")
		if err != nil {
			return nil, err
		}
		// The external port is optional here; we only need it if it's public
		var minExternalPort, maxExternalPort int32
		if port.External != "" {
			minExternalPort, maxExternalPort, err = parsePortRange(port.External, port.Name, "external")
			if err != nil {
				return nil, err
			}
		}
		if port.External != "" && maxInternalPort-minInternalPort != maxExternalPort-minExternalPort {
			return nil, fmt.Errorf("Port %s has mismatched internal and external port ranges %s and %s",
				port.Name, port.Internal, port.External)
		}

		portInfos, err := getPortInfo(port.Name, minInternalPort, maxInternalPort)
		if err != nil {
			return nil, err
		}
		for _, portInfo := range portInfos {
			result = append(result, v1.ContainerPort{
				Name:          portInfo.name,
				ContainerPort: portInfo.port,
				Protocol:      protocol,
			})
		}
	}

	return result, nil
}

// getVolumeMounts gets the list of volume mounts for a role
func getVolumeMounts(role *model.Role) []v1.VolumeMount {
	resultLen := len(role.Run.PersistentVolumes) + len(role.Run.SharedVolumes)
	result := make([]v1.VolumeMount, 0, resultLen)

	for _, volume := range role.Run.PersistentVolumes {
		result = append(result, v1.VolumeMount{
			Name:      volume.Tag,
			MountPath: volume.Path,
			ReadOnly:  false,
		})
	}

	for _, volume := range role.Run.SharedVolumes {
		result = append(result, v1.VolumeMount{
			Name:      volume.Tag,
			MountPath: volume.Path,
			ReadOnly:  false,
		})
	}

	return result
}

func getEnvVars(role *model.Role, defaults map[string]string, secrets SecretRefMap, createHelmChart bool) ([]v1.EnvVar, error) {
	configs, err := role.GetVariablesForRole()

	if err != nil {
		return nil, err
	}

	result := make([]v1.EnvVar, 0, len(configs))

	for _, config := range configs {
		// Secret CVs have special output, they refer to a K8s
		// secret for their value instead of storing it
		// directly.
		if config.Secret {
			result = append(result, v1.EnvVar{
				Name: config.Name,
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: secrets[config.Name].Secret,
						},
						Key: secrets[config.Name].Key,
					},
				},
			})
			continue
		}

		var stringifiedValue string
		if createHelmChart {
			required := ""
			if config.Required {
				required = fmt.Sprintf(`required "%s configuration missing" `, config.Name)
			}
			stringifiedValue = fmt.Sprintf("{{ %s.Values.env.%s | quote }}", required, config.Name)
		} else {
			var ok bool
			ok, stringifiedValue = ConfigValue(config, defaults)
			if !ok {
				continue
			}
		}

		result = append(result, v1.EnvVar{
			Name:  config.Name,
			Value: stringifiedValue,
		})
	}

	result = append(result, v1.EnvVar{
		Name: "KUBERNETES_NAMESPACE",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	})

	return result, nil
}

// ConfigValue fetches the value of config variable (should this live somewhere else?)
func ConfigValue(config *model.ConfigurationVariable, defaults map[string]string) (bool, string) {
	var value interface{}

	value = config.Default

	if defaultValue, ok := defaults[config.Name]; ok {
		value = defaultValue
	}

	if value == nil {
		return false, ""
	}

	var stringifiedValue string
	if valueAsString, ok := value.(string); ok {
		var err error
		stringifiedValue, err = strconv.Unquote(fmt.Sprintf(`"%s"`, valueAsString))
		if err != nil {
			stringifiedValue = valueAsString
		}
	} else {
		stringifiedValue = fmt.Sprintf("%v", value)
	}

	return true, stringifiedValue
}

func getSecurityContext(role *model.Role) *v1.SecurityContext {
	privileged := true

	sc := &v1.SecurityContext{}
	for _, c := range role.Run.Capabilities {
		c = strings.ToUpper(c)
		if c == "ALL" {
			sc.Privileged = &privileged
			return sc
		}
		if sc.Capabilities == nil {
			sc.Capabilities = &v1.Capabilities{}
		}
		sc.Capabilities.Add = append(sc.Capabilities.Add, v1.Capability(c))
	}

	if sc.Capabilities == nil {
		return nil
	}
	return sc
}

func getContainerLivenessProbe(role *model.Role) (*v1.Probe, error) {
	// Ref https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/#configuring-probes
	// Ref vendor/k8s.io/client-go/pkg/api/v1/types.go (1297ff)
	//
	// InitialDelaySeconds -
	// TimeoutSeconds      - 1, min 1
	// PeriodSeconds       - 10, min 1 (interval between probes)
	// SuccessThreshold    - 1 (default, must be this value for liveness probe)
	// FailureThreshold    - 3, min 1

	if role.Run == nil {
		return nil, nil
	}

	probe := &v1.Probe{}

	if role.Run.HealthCheck != nil &&
		role.Run.HealthCheck.Liveness != nil {

		p, err := configureContainerProbe(role, "liveness", role.Run.HealthCheck.Liveness, probe)

		// Liveness-specific post-processing of fields.

		// The SuccessThreshold defaults to and cannot be
		// anything but 1 for a liveness probe according to
		// k8s docs. Force this default over anything the
		// manifest may have done in configureCotnainerProbe.
		probe.SuccessThreshold = 0

		if probe.InitialDelaySeconds == 0 {
			// Our standard default
			probe.InitialDelaySeconds = 600
		}

		// Bail early on error or when configured with custom action
		if p != nil || err != nil {
			return p, err
		}

		// p not set => probe is configured witohut custom action.
		// Fall through to the type-specific action setup below.
	}

	switch role.Type {
	case model.RoleTypeBosh:
		if probe.InitialDelaySeconds == 0 {
			// Our standard default
			probe.InitialDelaySeconds = 600
		}
		probe.Handler = v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(monitPort),
			},
		}

		return probe, nil
	default:
		return nil, nil
	}
}

func getContainerReadinessProbe(role *model.Role) (*v1.Probe, error) {
	if role.Run == nil {
		return nil, nil
	}

	probe := &v1.Probe{}

	if role.Run.HealthCheck != nil &&
		role.Run.HealthCheck.Readiness != nil {

		p, err := configureContainerProbe(role, "readiness", role.Run.HealthCheck.Readiness, probe)

		// Bail early on error or when configured with custom action
		if p != nil || err != nil {
			return p, err
		}

		// p not set => probe is configured without custom action.
		// Fall through to the type-specific action setup below.
	}
	switch role.Type {
	case model.RoleTypeBosh:
		var readinessPort *model.RoleRunExposedPort
		for _, port := range role.Run.ExposedPorts {
			if strings.ToUpper(port.Protocol) != "TCP" {
				continue
			}
			if readinessPort == nil {
				readinessPort = port
			}
		}
		if readinessPort == nil {
			return nil, nil
		}
		probePort, _, err := parsePortRange(readinessPort.Internal, readinessPort.Name, "internal")
		if err != nil {
			return nil, err
		}
		probe.Handler = v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(int(probePort)),
			},
		}
		return probe, nil
	default:
		return nil, nil
	}
}

func configureContainerProbe(role *model.Role, probeName string, roleProbe *model.HealthProbe, probe *v1.Probe) (*v1.Probe, error) {
	// Ref https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/#configuring-probes
	// Ref vendor/k8s.io/client-go/pkg/api/v1/types.go (1297ff)
	//
	// InitialDelaySeconds -
	// TimeoutSeconds      - 1, min 1
	// PeriodSeconds       - 10, min 1 (interval between probes)
	// SuccessThreshold    - 1, min 1 (must be 1 for liveness probe)
	// FailureThreshold    - 3, min 1

	probe.InitialDelaySeconds = roleProbe.InitialDelay
	probe.TimeoutSeconds = roleProbe.Timeout
	probe.PeriodSeconds = roleProbe.Period
	probe.SuccessThreshold = roleProbe.SuccessThreshold
	probe.FailureThreshold = roleProbe.FailureThreshold

	if roleProbe.URL != "" {
		return getContainerURLProbe(role, probeName, roleProbe, probe)
	}

	if roleProbe.Port != 0 {
		probe.Handler = v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(int(roleProbe.Port)),
			},
		}
		return probe, nil
	}

	if len(roleProbe.Command) > 0 {
		probe.Handler = v1.Handler{
			Exec: &v1.ExecAction{
				Command: roleProbe.Command,
			},
		}
		return probe, nil
	}

	// Configured, but not a custom action.
	return nil, nil
}

func getContainerURLProbe(role *model.Role, probeName string, roleProbe *model.HealthProbe, probe *v1.Probe) (*v1.Probe, error) {
	probeURL, err := url.Parse(roleProbe.URL)
	if err != nil {
		return nil, fmt.Errorf("Invalid %s URL health check for %s: %s", probeName, role.Name, err)
	}

	var scheme v1.URIScheme
	var port intstr.IntOrString
	switch strings.ToUpper(probeURL.Scheme) {
	case string(v1.URISchemeHTTP):
		scheme = v1.URISchemeHTTP
		port = intstr.FromInt(80)
	case string(v1.URISchemeHTTPS):
		scheme = v1.URISchemeHTTPS
		port = intstr.FromInt(443)
	default:
		return nil, fmt.Errorf("Health check for %s has unsupported URI scheme \"%s\"", role.Name, probeURL.Scheme)
	}

	host := probeURL.Host
	// url.URL will have a `Host` of `example.com:8080`, but kubernetes takes a separate `Port` field
	if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
		portNum, err := strconv.Atoi(host[colonIndex+1:])
		if err != nil {
			return nil, fmt.Errorf("Failed to get URL port for health check for %s: invalid host \"%s\"", role.Name, probeURL.Host)
		}
		port = intstr.FromInt(portNum)
		host = host[:colonIndex]
	}
	if host == "container-ip" {
		// Special case to use the pod IP instead; this is run from outside the pod
		host = ""
	}

	var headers []v1.HTTPHeader
	if probeURL.User != nil {
		headers = append(headers, v1.HTTPHeader{
			Name:  "Authorization",
			Value: base64.StdEncoding.EncodeToString([]byte(probeURL.User.String())),
		})
	}
	for key, value := range roleProbe.Headers {
		headers = append(headers, v1.HTTPHeader{
			Name:  http.CanonicalHeaderKey(key),
			Value: value,
		})
	}

	path := probeURL.Path

	if probeURL.RawQuery != "" {
		path += "?" + probeURL.RawQuery
	}
	// probeURL.Fragment should not be sent to the server, so we ignore it here

	probe.Handler = v1.Handler{
		HTTPGet: &v1.HTTPGetAction{
			Host:        host,
			Port:        port,
			Path:        path,
			Scheme:      scheme,
			HTTPHeaders: headers,
		},
	}

	return probe, nil
}

//metadata:
//  name: wordpress-mysql
//  labels:
//    app: wordpress
//spec:
//  strategy:
//    type: Recreate
//  template:
//    metadata:
//      labels:
//        app: wordpress
//        tier: mysql
//    spec:
//      containers:
//      - image: mysql:5.6
//        name: mysql
//        env:
//          # $ kubectl create secret generic mysql-pass --from-file=password.txt
//          # make sure password.txt does not have a trailing newline
//        - name: MYSQL_ROOT_PASSWORD
//          valueFrom:
//            secretKeyRef:
//              name: mysql-pass
//              key: password.txt
//        ports:
//        - containerPort: 3306
//          name: mysql
//        volumeMounts:
//        - name: mysql-persistent-storage
//          mountPath: /var/lib/mysql
//      volumes:
//      - name: mysql-persistent-storage
//        persistentVolumeClaim:
//          claimName: mysql-pv-claim
