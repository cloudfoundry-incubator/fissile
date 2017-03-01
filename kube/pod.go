package kube

import (
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hpcloud/fissile/builder"
	"github.com/hpcloud/fissile/model"

	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
)

// monitPort is the port monit runs on in the pods
const monitPort = 2289

// NewPodTemplate creates a new pod template spec for a given role, as well as
// any objects it depends on
func NewPodTemplate(role *model.Role, settings *ExportSettings) (v1.PodTemplateSpec, error) {

	vars, err := getEnvVars(role, settings.Defaults)
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
					Image:           getContainerImageName(role, settings),
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

	livenessProbe := getContainerLivenessProbe(role)
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

// getContainerImageName returns the name of the docker image to use for a role
func getContainerImageName(role *model.Role, settings *ExportSettings) string {
	devImageName := builder.GetRoleDevImageName(settings.Repository, role, role.GetRoleDevVersion())
	imageName := devImageName

	if settings.Organization != "" && settings.Registry != "" {
		imageName = fmt.Sprintf("%s/%s/%s", settings.Registry, settings.Organization, devImageName)
	} else if settings.Organization != "" {
		imageName = fmt.Sprintf("%s/%s", settings.Organization, devImageName)
	} else if settings.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", settings.Registry, devImageName)
	}

	return imageName
}

// getContainerPorts returns a list of ports for a role
func getContainerPorts(role *model.Role) ([]v1.ContainerPort, error) {
	result := make([]v1.ContainerPort, len(role.Run.ExposedPorts))

	for i, port := range role.Run.ExposedPorts {
		var protocol v1.Protocol

		switch strings.ToLower(port.Protocol) {
		case "tcp":
			protocol = v1.ProtocolTCP
		case "udp":
			protocol = v1.ProtocolUDP
		}

		// We may need to fixup the port name.  It must:
		// - not be empty
		// - be no more than 15 characters long
		// - consist only of letters, digits, or hyphen
		// - start and end with a letter or a digit
		// - there can not be consecutive hyphens
		nameChars := make([]rune, 0, len(port.Name))
		for _, ch := range port.Name {
			switch {
			case ch >= 'A' && ch <= 'Z':
				nameChars = append(nameChars, ch)
			case ch >= 'a' && ch <= 'z':
				nameChars = append(nameChars, ch)
			case ch >= '0' && ch <= '9':
				nameChars = append(nameChars, ch)
			case ch == '-':
				if len(nameChars) == 0 {
					// Skip leading hyphens
					continue
				}
				if nameChars[len(nameChars)-1] == '-' {
					// Skip consecutive hyphens
					continue
				}
				nameChars = append(nameChars, ch)
			}
		}
		// Strip trailing hyphens
		for len(nameChars) > 0 && nameChars[len(nameChars)-1] == '-' {
			nameChars = nameChars[:len(nameChars)-1]
		}
		name := string(nameChars)
		if name == "" {
			return nil, fmt.Errorf("Port name %s does not contain any letters or digits", port.Name)
		}

		if len(name) > 15 {
			// Kubernetes doesn't like names that long
			name = fmt.Sprintf("%s%x", name[:7], crc32.ChecksumIEEE([]byte(name)))
		}
		result[i] = v1.ContainerPort{
			Name:          name,
			ContainerPort: int32(port.Internal),
			Protocol:      protocol,
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

func getEnvVars(role *model.Role, defaults map[string]string) ([]v1.EnvVar, error) {
	configs, err := role.GetVariablesForRole()

	if err != nil {
		return nil, err
	}

	result := make([]v1.EnvVar, 0, len(configs))

	for _, config := range configs {
		var value interface{}

		value = config.Default

		if defaultValue, ok := defaults[config.Name]; ok {
			value = defaultValue
		}

		if value == nil {
			continue
		}

		var stringifiedValue string
		if valueAsString, ok := value.(string); ok {
			stringifiedValue, err = strconv.Unquote(fmt.Sprintf(`"%s"`, valueAsString))
			if err != nil {
				stringifiedValue = valueAsString
			}
		} else {
			stringifiedValue = fmt.Sprintf("%v", value)
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

func getContainerLivenessProbe(role *model.Role) *v1.Probe {
	switch role.Type {
	case model.RoleTypeBosh:
		return &v1.Probe{
			Handler: v1.Handler{
				TCPSocket: &v1.TCPSocketAction{
					Port: intstr.FromInt(monitPort),
				},
			},
			// TODO: make this configurable (figure out where the knob should live)
			InitialDelaySeconds: 180,
		}
	default:
		return nil
	}
}

func getContainerReadinessProbe(role *model.Role) (*v1.Probe, error) {
	if role.Run == nil {
		return nil, nil
	}
	if role.Run.HealthCheck != nil {
		if role.Run.HealthCheck.URL != "" {
			return getContainerURLReadinessProbe(role)
		}
		if role.Run.HealthCheck.Port != 0 {
			return &v1.Probe{
				Handler: v1.Handler{
					TCPSocket: &v1.TCPSocketAction{
						Port: intstr.FromInt(int(role.Run.HealthCheck.Port)),
					},
				},
			}, nil
		}
		if len(role.Run.HealthCheck.Command) > 0 {
			return &v1.Probe{
				Handler: v1.Handler{
					Exec: &v1.ExecAction{
						Command: role.Run.HealthCheck.Command,
					},
				},
			}, nil
		}
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
		return &v1.Probe{
			Handler: v1.Handler{
				TCPSocket: &v1.TCPSocketAction{
					Port: intstr.FromInt(int(readinessPort.Internal)),
				},
			},
		}, nil
	default:
		return nil, nil
	}
}

func getContainerURLReadinessProbe(role *model.Role) (*v1.Probe, error) {
	probeURL, err := url.Parse(role.Run.HealthCheck.URL)
	if err != nil {
		return nil, fmt.Errorf("Invalid URL health check for %s: %s", role.Name, err)
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
	for key, value := range role.Run.HealthCheck.Headers {
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

	return &v1.Probe{
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Host:        host,
				Port:        port,
				Path:        path,
				Scheme:      scheme,
				HTTPHeaders: headers,
			},
		},
	}, nil
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
