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
	"github.com/SUSE/fissile/util"
)

// defaultInitialDelaySeconds is the default initial delay for liveness probes
const defaultInitialDelaySeconds = 600

// NewPodTemplate creates a new pod template spec for a given role, as well as
// any objects it depends on
func NewPodTemplate(role *model.Role, settings ExportSettings, grapher util.ModelGrapher) (helm.Node, error) {
	if role.Run == nil {
		return nil, fmt.Errorf("Role %s has no run information", role.Name)
	}

	roleName := strings.Replace(strings.ToLower(role.Name), "_", "-", -1)
	roleVarName := makeVarName(roleName)

	vars, err := getEnvVars(role, settings)
	if err != nil {
		return nil, err
	}

	var resources helm.Node
	var requests *helm.Mapping
	var limits *helm.Mapping

	if settings.UseMemoryLimits || settings.UseCPULimits {
		requests = helm.NewMapping()
		limits = helm.NewMapping()
		resources = helm.NewMapping("requests", requests, "limits", limits)
	}

	if settings.UseMemoryLimits {
		if settings.CreateHelmChart {
			requests.Add("memory",
				helm.NewNode(fmt.Sprintf("{{ int .Values.sizing.%s.memory.request }}Mi", roleVarName),
					helm.Block(fmt.Sprintf("if and .Values.sizing.memory.requests .Values.sizing.%s.memory.request", roleVarName))))
			limits.Add("memory",
				helm.NewNode(fmt.Sprintf("{{ int .Values.sizing.%s.memory.limit }}Mi", roleVarName),
					helm.Block(fmt.Sprintf("if and .Values.sizing.memory.limits .Values.sizing.%s.memory.limit", roleVarName))))
		} else {
			if role.Run.Memory != nil {
				if role.Run.Memory.Request != nil {
					requests.Add("memory", fmt.Sprintf("%dMi", *role.Run.Memory.Request))
				}
				if role.Run.Memory.Limit != nil {
					limits.Add("memory", fmt.Sprintf("%dMi", *role.Run.Memory.Limit))
				}
			}
		}
	}
	if settings.UseCPULimits {
		if settings.CreateHelmChart {
			requests.Add("cpu",
				helm.NewNode(fmt.Sprintf("{{ int .Values.sizing.%s.cpu.request }}m", roleVarName),
					helm.Block(fmt.Sprintf("if and .Values.sizing.cpu.requests .Values.sizing.%s.cpu.request", roleVarName))))
			limits.Add("cpu",
				helm.NewNode(fmt.Sprintf("{{ int .Values.sizing.%s.cpu.limit }}m", roleVarName),
					helm.Block(fmt.Sprintf("if and .Values.sizing.cpu.limits .Values.sizing.%s.cpu.limit", roleVarName))))
		} else {
			if role.Run.CPU != nil {
				if role.Run.CPU.Request != nil {
					requests.Add("cpu", fmt.Sprintf("%dm", int(*role.Run.CPU.Request*1000+0.5)))
				}
				if role.Run.CPU.Limit != nil {
					limits.Add("cpu", fmt.Sprintf("%dm", int(*role.Run.CPU.Limit*1000+0.5)))
				}
			}
		}
	}

	securityContext := getSecurityContext(role)
	ports, err := getContainerPorts(role, settings)
	if err != nil {
		return nil, err
	}
	image, err := getContainerImageName(role, settings, grapher)
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
	container.Add("lifecycle",
		helm.NewMapping("preStop",
			helm.NewMapping("exec",
				helm.NewMapping("command",
					[]string{"/opt/fissile/pre-stop.sh"}))))
	container.Sort()

	imagePullSecrets := helm.NewMapping("name", "registry-credentials")

	spec := helm.NewMapping()
	spec.Add("containers", helm.NewList(container))
	spec.Add("imagePullSecrets", helm.NewList(imagePullSecrets))
	spec.Add("dnsPolicy", "ClusterFirst")
	spec.Add("volumes", getNonClaimVolumes(role))
	spec.Add("restartPolicy", "Always")
	if role.Run.ServiceAccount != "" {
		// This role requires a custom service account
		block := helm.Block("")
		if settings.CreateHelmChart {
			block = helm.Block(authModeRBAC)
		}
		spec.Add("serviceAccountName", role.Run.ServiceAccount, block)
	}
	// BOSH can potentially have an infinite termination grace period; we don't
	// really trust that, so we'll just go with ten minutes and hope it's enough
	spec.Add("terminationGracePeriodSeconds", 600)
	spec.Sort()

	podTemplate := helm.NewMapping()
	meta := newObjectMeta(role.Name)
	if settings.CreateHelmChart {
		meta.Add("annotations", helm.NewMapping("checksum/config", `{{ include (print $.Template.BasePath "/secrets.yaml") . | sha256sum }}`))
	}
	podTemplate.Add("metadata", meta)
	podTemplate.Add("spec", spec)

	return podTemplate, nil
}

// NewPod creates a new Pod for the given role, as well as any objects it depends on
func NewPod(role *model.Role, settings ExportSettings, grapher util.ModelGrapher) (helm.Node, error) {
	podTemplate, err := NewPodTemplate(role, settings, grapher)
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
func getContainerImageName(role *model.Role, settings ExportSettings, grapher util.ModelGrapher) (string, error) {
	devVersion, err := role.GetRoleDevVersion(settings.Opinions, settings.TagExtra, settings.FissileVersion, grapher)
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
func getContainerPorts(role *model.Role, settings ExportSettings) (helm.Node, error) {
	var ports []helm.Node
	for _, port := range role.Run.ExposedPorts {
		if settings.CreateHelmChart && port.CountIsConfigurable {
			sizing := fmt.Sprintf(".Values.sizing.%s.ports.%s", makeVarName(role.Name), makeVarName(port.Name))

			fail := fmt.Sprintf(`{{ fail "%s.count must not exceed %d" }}`, sizing, port.Max)
			block := fmt.Sprintf("if gt (int %s.count) %d", sizing, port.Max)
			ports = append(ports, helm.NewNode(fail, helm.Block(block)))

			fail = fmt.Sprintf(`{{ fail "%s.count must be at least 1" }}`, sizing)
			block = fmt.Sprintf("if lt (int %s.count) 1", sizing)
			ports = append(ports, helm.NewNode(fail, helm.Block(block)))

			block = fmt.Sprintf("range $port := until (int %s.count)", sizing)
			newPort := helm.NewMapping()
			newPort.Set(helm.Block(block))
			newPort.Add("containerPort", fmt.Sprintf("{{ add %d $port }}", port.InternalPort))
			if port.Max > 1 {
				newPort.Add("name", fmt.Sprintf("%s-{{ $port }}", port.Name))
			} else {
				newPort.Add("name", port.Name)
			}
			newPort.Add("protocol", port.Protocol)
			ports = append(ports, newPort)
		} else {
			for portNumber := port.InternalPort; portNumber < port.InternalPort+port.Count; portNumber++ {
				newPort := helm.NewMapping()
				newPort.Add("containerPort", portNumber)
				if port.Max > 1 {
					newPort.Add("name", fmt.Sprintf("%s-%d", port.Name, portNumber))
				} else {
					newPort.Add("name", port.Name)
				}
				newPort.Add("protocol", port.Protocol)
				ports = append(ports, newPort)
			}
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
	for _, volume := range role.Run.Volumes {
		mount := helm.NewMapping("mountPath", volume.Path, "name", volume.Tag, "readOnly", false)
		if volume.Type == model.VolumeTypeHost {
			mount.Set(helm.Block("if .Values.kube.hostpath_available"))
		}
		mounts = append(mounts, mount)
	}
	if len(mounts) == 0 {
		return nil
	}
	return helm.NewNode(mounts)
}

const userSecretsName = "secrets"
const generatedSecretsName = "secrets-{{ .Chart.Version }}-{{ .Values.kube.secrets_generation_counter }}"

func makeSecretVar(name string, generated bool, modifiers ...helm.NodeModifier) helm.Node {
	secretKeyRef := helm.NewMapping("key", util.ConvertNameToKey(name))
	if generated {
		secretKeyRef.Add("name", generatedSecretsName)
	} else {
		secretKeyRef.Add("name", userSecretsName)
	}

	envVar := helm.NewMapping("name", name, "valueFrom", helm.NewMapping("secretKeyRef", secretKeyRef))
	envVar.Set(modifiers...)
	return envVar
}

// getNonClaimVolumes returns the list of pod volumes that are _not_ bound with volume claims
func getNonClaimVolumes(role *model.Role) helm.Node {
	var mounts []helm.Node
	for _, volume := range role.Run.Volumes {
		switch volume.Type {
		case model.VolumeTypeHost:
			hostPathInfo := helm.NewMapping("path", volume.Path)
			hostPathInfo.Add("type", "Directory", helm.Block(fmt.Sprintf("if (%s)", minKubeVersion(1, 8))))
			volumeEntry := helm.NewMapping("name", volume.Tag, "hostPath", hostPathInfo)
			volumeEntry.Set(helm.Block("if .Values.kube.hostpath_available"))
			mounts = append(mounts, volumeEntry)
		}
	}
	if len(mounts) == 0 {
		return nil
	}
	return helm.NewNode(mounts)
}

func getEnvVars(role *model.Role, settings ExportSettings) (helm.Node, error) {
	configs, err := role.GetVariablesForRole()
	if err != nil {
		return nil, err
	}

	sizingCountRegexp := regexp.MustCompile("^KUBE_SIZING_([A-Z][A-Z_]*)_COUNT$")
	sizingPortsRegexp := regexp.MustCompile("^KUBE_SIZING_([A-Z][A-Z_]*)_PORTS_([A-Z][A-Z_]*)_(MIN|MAX)$")

	var env []helm.Node
	for _, config := range configs {
		// KUBE_SIZING_role_COUNT
		match := sizingCountRegexp.FindStringSubmatch(config.Name)
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

		// KUBE_SIZING_role_PORTS_port_MIN/MAX
		match = sizingPortsRegexp.FindStringSubmatch(config.Name)
		if match != nil {
			roleName := strings.Replace(strings.ToLower(match[1]), "_", "-", -1)
			role := settings.RoleManifest.LookupRole(roleName)
			if role == nil {
				return nil, fmt.Errorf("Role %s for %s not found", roleName, config.Name)
			}
			if config.Secret {
				return nil, fmt.Errorf("%s must not be a secret variable", config.Name)
			}

			portName := strings.Replace(strings.ToLower(match[2]), "_", "-", -1)
			var port *model.RoleRunExposedPort
			for _, exposedPort := range role.Run.ExposedPorts {
				if (exposedPort.PortIsConfigurable || exposedPort.CountIsConfigurable) && exposedPort.Name == portName {
					port = exposedPort
					break
				}
			}
			if port == nil {
				return nil, fmt.Errorf("Role %s doesn't have a user configurable port %s", roleName, portName)
			}

			var value string
			if match[3] == "MIN" {
				value = strconv.Itoa(port.InternalPort)
			} else {
				if settings.CreateHelmChart {
					value = fmt.Sprintf("{{ add %d .Values.sizing.%s.ports.%s.count -1 | quote }}",
						port.InternalPort, makeVarName(roleName), makeVarName(portName))
				} else {
					value = strconv.Itoa(port.InternalPort + port.Count - 1)
				}
			}
			envVar := helm.NewMapping("name", config.Name, "value", value)
			env = append(env, envVar)
			continue
		}

		if config.Name == "KUBE_SECRETS_GENERATION_COUNTER" {
			value := "1"
			if settings.CreateHelmChart {
				value = "{{ .Values.kube.secrets_generation_counter | quote }}"
			}
			env = append(env, helm.NewMapping("name", config.Name, "value", value))
			continue
		}

		if config.Name == "KUBE_SECRETS_GENERATION_NAME" {
			value := "secrets-1"
			if settings.CreateHelmChart {
				value = generatedSecretsName
			}
			env = append(env, helm.NewMapping("name", config.Name, "value", value))
			continue
		}

		if config.Secret {
			if !settings.CreateHelmChart {
				env = append(env, makeSecretVar(config.Name, false))
			} else {
				if config.Immutable && config.Generator != nil {
					// Users cannot override immutable secrets that are generated
					env = append(env, makeSecretVar(config.Name, true))
				} else if config.Generator == nil {
					env = append(env, makeSecretVar(config.Name, false))
				} else {
					// Generated secrets can be overridden by the user (unless immutable)
					block := helm.Block(fmt.Sprintf("if not .Values.secrets.%s", config.Name))
					env = append(env, makeSecretVar(config.Name, true, block))

					block = helm.Block(fmt.Sprintf("if .Values.secrets.%s", config.Name))
					env = append(env, makeSecretVar(config.Name, false, block))
				}
			}
			continue
		}

		var stringifiedValue string
		if settings.CreateHelmChart && config.Type == model.CVTypeUser {
			required := ""
			if config.Required {
				required = fmt.Sprintf(`required "%s configuration missing" `, config.Name)
			}
			stringifiedValue = fmt.Sprintf("{{ %s.Values.env.%s | quote }}", required, config.Name)
		} else {
			var ok bool
			ok, stringifiedValue = config.Value(settings.Defaults)
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

	if role.Run.HealthCheck != nil && role.Run.HealthCheck.Liveness != nil {
		probe, complete, err := configureContainerProbe(role, "liveness", role.Run.HealthCheck.Liveness)

		if probe.Get("initialDelaySeconds").String() == "0" {
			probe.Add("initialDelaySeconds", defaultInitialDelaySeconds)
		}
		if complete || err != nil {
			return probe, err
		}
	}

	// No custom probes; we don't have a default one either.
	return nil, nil
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
		if port.Protocol == "TCP" {
			readinessPort = port
			break
		}
	}
	if readinessPort == nil {
		return nil, nil
	}

	if probe == nil {
		probe = helm.NewMapping()
	}
	probe.Add("tcpSocket", helm.NewMapping("port", readinessPort.InternalPort))
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
