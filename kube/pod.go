package kube

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SUSE/fissile/builder"
	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// monitPort is the port monit runs on in the pods
const monitPort = 2289

// defaultInitialDelaySeconds is the default initial delay for liveness probes
const defaultInitialDelaySeconds = 600

// NewPodTemplate creates a new pod template spec for a given role, as well as
// any objects it depends on
func NewPodTemplate(role *model.Role, settings *ExportSettings) (helm.Node, error) {
	if role.Run == nil {
		return nil, fmt.Errorf("Role %s has no run information", role.Name)
	}

	vars, err := getEnvVars(role, settings.Defaults, settings.Secrets, settings)
	if err != nil {
		return nil, err
	}

	var resources helm.Node
	if settings.UseMemoryLimits {
		resources = helm.NewMapping("requests", helm.NewMapping("memory", fmt.Sprintf("%dMi", role.Run.Memory)))
	}

	securityContext := getSecurityContext(role)
	ports, err := getContainerPorts(role)
	if err != nil {
		return nil, err
	}
	image, err := getContainerImageName(role, settings)
	if err != nil {
		return nil, err
	}
	livenessProbe, err := getContainerLivenessProbe(role)
	if err != nil {
		return nil, err
	}
	readinessProbe, err := getContainerReadinessProbe(role)
	if err != nil {
		return nil, err
	}

	container := helm.NewMapping()
	container.Add("name", role.Name)
	container.Add("image", image)
	container.Add("ports", ports)
	container.Add("volumeMounts", getVolumeMounts(role))
	container.Add("env", vars)
	container.Add("resources", resources)
	container.Add("securityContext", securityContext)
	container.Add("livenessProbe", livenessProbe)
	container.Add("readinessProbe", readinessProbe)
	container.Sort()

	imagePullSecrets := helm.NewMapping()
	imagePullSecrets.Add("name", "registry-credentials")

	spec := helm.NewMapping()
	spec.Add("containers", helm.NewList(container))
	spec.Add("imagePullSecrets", helm.NewList(imagePullSecrets))
	spec.Add("dnsPolicy", "ClusterFirst")
	spec.Add("restartPolicy", "Always")
	spec.Sort()

	podTemplate := helm.NewMapping()
	podTemplate.Add("metadata", newObjectMeta(role.Name))
	podTemplate.Add("spec", spec)

	return podTemplate, nil
}

// NewPod creates a new Pod for the given role, as well as any objects it depends on
func NewPod(role *model.Role, settings *ExportSettings) (helm.Node, error) {
	podTemplate, err := NewPodTemplate(role, settings)
	if err != nil {
		return nil, err
	}

	// Pod must have a restart policy that isn't "always"
	switch role.Run.FlightStage {
	case model.FlightStageManual:
		podTemplate.Get("spec", "restartPolicy").SetValue("Never")
	case model.FlightStageFlight, model.FlightStagePreFlight, model.FlightStagePostFlight:
		podTemplate.Get("spec", "restartPolicy").SetValue("OnFailure")
	default:
		return nil, fmt.Errorf("Role %s has unexpected flight stage %s", role.Name, role.Run.FlightStage)
	}

	pod := newKubeConfig("v1", "Pod", role.Name, helm.Comment(role.GetLongDescription()))
	pod.Add("spec", podTemplate.Get("spec"))

	return pod.Sort(), nil
}

// getContainerImageName returns the name of the docker image to use for a role
func getContainerImageName(role *model.Role, settings *ExportSettings) (string, error) {
	devVersion, err := role.GetRoleDevVersion(settings.Opinions, settings.TagExtra, settings.FissileVersion)
	if err != nil {
		return "", err
	}

	var imageName string
	if settings.CreateHelmChart {
		registry := "{{ .Values.kube.registry.hostname }}"
		org := "{{ .Values.kube.organization }}"
		imageName = builder.GetRoleDevImageName(registry, org, settings.Repository, role, devVersion)
	} else {
		imageName = builder.GetRoleDevImageName(settings.Registry, settings.Organization, settings.Repository, role, devVersion)
	}

	return imageName, nil
}

// getContainerPorts returns a list of ports for a role
func getContainerPorts(role *model.Role) (helm.Node, error) {
	var ports []helm.Node
	for _, port := range role.Run.ExposedPorts {
		// Convert port range specifications to port numbers
		minInternalPort, maxInternalPort, err := parsePortRange(port.Internal, port.Name, "internal")
		if err != nil {
			return nil, err
		}
		// The external port is optional here; we only need it if it's public
		var minExternalPort, maxExternalPort int
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
			newPort := helm.NewMapping()
			newPort.Add("containerPort", portInfo.port)
			newPort.Add("name", portInfo.name)
			newPort.Add("protocol", strings.ToUpper(port.Protocol))
			ports = append(ports, newPort)
		}
	}
	if len(ports) == 0 {
		return nil, nil
	}
	return helm.NewNode(ports), nil
}

// getVolumeMounts gets the list of volume mounts for a role
func getVolumeMounts(role *model.Role) helm.Node {
	var mounts []helm.Node
	for _, volume := range role.Run.PersistentVolumes {
		mounts = append(mounts, helm.NewMapping("mountPath", volume.Path, "name", volume.Tag, "readOnly", false))
	}
	for _, volume := range role.Run.SharedVolumes {
		mounts = append(mounts, helm.NewMapping("mountPath", volume.Path, "name", volume.Tag, "readOnly", false))
	}
	if len(mounts) == 0 {
		return nil
	}
	return helm.NewNode(mounts)
}

func getEnvVars(role *model.Role, defaults map[string]string, secrets SecretRefMap, settings *ExportSettings) (helm.Node, error) {
	configs, err := role.GetVariablesForRole()
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile("^KUBE_SIZING_([A-Z][A-Z_]*)_COUNT$")

	var env []helm.Node
	for _, config := range configs {
		match := re.FindStringSubmatch(config.Name)
		if match != nil {
			roleName := strings.Replace(strings.ToLower(match[1]), "_", "-", -1)
			role := settings.RoleManifest.LookupRole(roleName)
			if role == nil {
				return nil, fmt.Errorf("Role %s for %s not found", roleName, config.Name)
			}
			if config.Secret {
				return nil, fmt.Errorf("%s must not be a secret variable", config.Name)
			}
			if settings.CreateHelmChart {
				value := fmt.Sprintf("{{ .Values.sizing.%s.count | quote }}", makeVarName(roleName))
				envVar := helm.NewMapping("name", config.Name, "value", value)
				env = append(env, envVar)
			} else {
				envVar := helm.NewMapping("name", config.Name, "value", strconv.Itoa(role.Run.Scaling.Min))
				env = append(env, envVar)
			}
			continue
		}

		if config.Secret {
			secretKeyRef := helm.NewMapping("key", secrets[config.Name].Key, "name", secrets[config.Name].Secret)

			envVar := helm.NewMapping("name", config.Name)
			envVar.Add("valueFrom", helm.NewMapping("secretKeyRef", secretKeyRef))

			env = append(env, envVar)
			continue
		}

		var stringifiedValue string
		if settings.CreateHelmChart {
			required := ""
			if config.Required {
				required = fmt.Sprintf(`required "%s configuration missing" `, config.Name)
			}
			stringifiedValue = fmt.Sprintf("{{ %s.Values.env.%s | quote }}", required, config.Name)
		} else {
			var ok bool
			ok, stringifiedValue = config.Value(defaults)
			if !ok {
				// Ignore config vars that don't have a default value
				continue
			}
		}
		env = append(env, helm.NewMapping("name", config.Name, "value", stringifiedValue))
	}

	fieldRef := helm.NewMapping("fieldPath", "metadata.namespace")

	envVar := helm.NewMapping("name", "KUBERNETES_NAMESPACE")
	envVar.Add("valueFrom", helm.NewMapping("fieldRef", fieldRef))

	env = append(env, envVar)

	sort.Slice(env[:], func(i, j int) bool {
		return env[i].Get("name").String() < env[j].Get("name").String()
	})
	return helm.NewNode(env), nil
}

func getSecurityContext(role *model.Role) helm.Node {
	var capabilities []string
	for _, cap := range role.Run.Capabilities {
		cap = strings.ToUpper(cap)
		if cap == "ALL" {
			return helm.NewMapping("privileged", true)
		}
		capabilities = append(capabilities, cap)
	}
	if len(capabilities) == 0 {
		return nil
	}
	return helm.NewMapping("capabilities", helm.NewMapping("add", helm.NewNode(capabilities)))
}

func getContainerLivenessProbe(role *model.Role) (helm.Node, error) {
	if role.Run == nil {
		return nil, nil
	}

	var probe *helm.Mapping
	if role.Run.HealthCheck != nil && role.Run.HealthCheck.Liveness != nil {
		var complete bool
		var err error
		probe, complete, err = configureContainerProbe(role, "liveness", role.Run.HealthCheck.Liveness)

		if probe.Get("initialDelaySeconds").String() == "0" {
			probe.Add("initialDelaySeconds", defaultInitialDelaySeconds)
		}
		if complete || err != nil {
			return probe, err
		}
	}
	if role.Type != model.RoleTypeBosh {
		return nil, nil
	}

	if probe == nil {
		probe = helm.NewMapping()
	}
	if probe.Get("initialDelaySeconds") == nil {
		probe.Add("initialDelaySeconds", defaultInitialDelaySeconds)
	}
	probe.Add("tcpSocket", helm.NewMapping("port", monitPort))
	return probe.Sort(), nil
}

func getContainerReadinessProbe(role *model.Role) (helm.Node, error) {
	if role.Run == nil {
		return nil, nil
	}

	var probe *helm.Mapping
	if role.Run.HealthCheck != nil && role.Run.HealthCheck.Readiness != nil {
		var complete bool
		var err error
		probe, complete, err = configureContainerProbe(role, "readiness", role.Run.HealthCheck.Readiness)
		if complete || err != nil {
			return probe, err
		}
	}
	if role.Type != model.RoleTypeBosh {
		return nil, nil
	}

	var readinessPort *model.RoleRunExposedPort
	for _, port := range role.Run.ExposedPorts {
		if strings.ToUpper(port.Protocol) == "TCP" {
			readinessPort = port
			break
		}
	}
	if readinessPort == nil {
		return nil, nil
	}
	probePort, _, err := parsePortRange(readinessPort.Internal, readinessPort.Name, "internal")
	if err != nil {
		return nil, err
	}

	if probe == nil {
		probe = helm.NewMapping()
	}
	probe.Add("tcpSocket", helm.NewMapping("port", probePort))
	return probe.Sort(), nil
}

func configureContainerProbe(role *model.Role, probeName string, roleProbe *model.HealthProbe) (*helm.Mapping, bool, error) {
	// InitialDelaySeconds -
	// TimeoutSeconds      - 1, min 1
	// PeriodSeconds       - 10, min 1 (interval between probes)
	// SuccessThreshold    - 1, min 1 (must be 1 for liveness probe)
	// FailureThreshold    - 3, min 1

	probe := helm.NewMapping()
	probe.Add("initialDelaySeconds", roleProbe.InitialDelay)
	probe.Add("timeoutSeconds", roleProbe.Timeout)
	probe.Add("periodSeconds", roleProbe.Period)
	probe.Add("successThreshold", roleProbe.SuccessThreshold)
	probe.Add("failureThreshold", roleProbe.FailureThreshold)

	if roleProbe.URL != "" {
		urlProbe, err := getContainerURLProbe(role, probeName, roleProbe)
		if err == nil {
			probe.Merge(urlProbe.(*helm.Mapping))
		}
		return probe.Sort(), true, err
	}
	if roleProbe.Port != 0 {
		probe.Add("tcpSocket", helm.NewMapping("port", roleProbe.Port))
		return probe.Sort(), true, nil
	}
	if len(roleProbe.Command) > 0 {
		probe.Add("exec", helm.NewMapping("command", helm.NewNode(roleProbe.Command)))
		return probe.Sort(), true, nil
	}

	// Configured, but not a custom action.
	return probe.Sort(), false, nil
}

func getContainerURLProbe(role *model.Role, probeName string, roleProbe *model.HealthProbe) (helm.Node, error) {
	probeURL, err := url.Parse(roleProbe.URL)
	if err != nil {
		return nil, fmt.Errorf("Invalid %s URL health check for %s: %s", probeName, role.Name, err)
	}

	var port int
	scheme := strings.ToUpper(probeURL.Scheme)

	switch scheme {
	case "HTTP":
		port = 80
	case "HTTPS":
		port = 443
	default:
		return nil, fmt.Errorf("Health check for %s has unsupported URI scheme \"%s\"", role.Name, probeURL.Scheme)
	}

	host := probeURL.Host
	// url.URL will have a `Host` of `example.com:8080`, but kubernetes takes a separate `Port` field
	if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
		port, err = strconv.Atoi(host[colonIndex+1:])
		if err != nil {
			return nil, fmt.Errorf("Failed to get URL port for health check for %s: invalid host \"%s\"", role.Name, probeURL.Host)
		}
		host = host[:colonIndex]
	}

	httpGet := helm.NewMapping("scheme", scheme, "port", port)
	// Set the host address, unless it's the special case to use the pod IP instead
	if host != "container-ip" {
		httpGet.Add("host", host)
	}

	var headers []helm.Node
	if probeURL.User != nil {
		headers = append(headers, helm.NewMapping(
			"name", "Authorization",
			"value", base64.StdEncoding.EncodeToString([]byte(probeURL.User.String())),
		))
	}
	for key, value := range roleProbe.Headers {
		headers = append(headers, helm.NewMapping(
			"name", http.CanonicalHeaderKey(key),
			"value", value,
		))
	}
	if len(headers) > 0 {
		httpGet.Add("httpHeaders", helm.NewNode(headers))
	}

	path := probeURL.Path
	if probeURL.RawQuery != "" {
		path += "?" + probeURL.RawQuery
	}
	// probeURL.Fragment should not be sent to the server, so we ignore it here
	httpGet.Add("path", path)
	httpGet.Sort()

	return helm.NewMapping("httpGet", httpGet), nil
}
