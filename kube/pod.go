package kube

import (
	"fmt"
	"strings"

	"github.com/hpcloud/fissile/builder"
	"github.com/hpcloud/fissile/model"

	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/v1"
)

// NewPodTemplate creates a new pod template spec for a given role, as well as
// any objects it depends on
func NewPodTemplate(role *model.Role, settings *KubeExportSettings) (v1.PodTemplateSpec, error) {

	vars, err := getEnvVars(role, settings.Defaults)
	if err != nil {
		return v1.PodTemplateSpec{}, err
	}

	devImageName := builder.GetRoleDevImageName(settings.Repository, role, role.GetRoleDevVersion())
	imageName := devImageName

	if settings.Organization != "" && settings.Registry != "" {
		imageName = fmt.Sprintf("%s/%s/%s", settings.Registry, settings.Organization, devImageName)
	} else if settings.Organization != "" {
		imageName = fmt.Sprintf("%s/%s", settings.Organization, devImageName)
	} else if settings.Registry != "" {
		imageName = fmt.Sprintf("%s/%s", settings.Registry, devImageName)
	}

	return v1.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Name: role.Name,
			Labels: map[string]string{
				RoleNameLabel: role.Name,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				v1.Container{
					Name:         role.Name,
					Image:        imageName,
					Ports:        getContainerPorts(role),
					VolumeMounts: getVolumeMounts(role),
					Env:          vars,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", role.Run.Memory)),
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyAlways,
			DNSPolicy:     v1.DNSClusterFirst,
		},
	}, nil
}

// getContainerPorts returns a list of ports for a role
func getContainerPorts(role *model.Role) []v1.ContainerPort {
	result := make([]v1.ContainerPort, len(role.Run.ExposedPorts))

	for i, port := range role.Run.ExposedPorts {
		var protocol v1.Protocol

		switch strings.ToLower(port.Protocol) {
		case "tcp":
			protocol = v1.ProtocolTCP
		case "udp":
			protocol = v1.ProtocolUDP
		}

		result[i] = v1.ContainerPort{
			Name:          port.Name,
			ContainerPort: int32(port.Internal),
			HostPort:      int32(port.External),
			Protocol:      protocol,
		}
	}

	return result
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

	result := make([]v1.EnvVar, len(configs))

	for i, config := range configs {
		var value interface{}

		value = config.Default

		if defaultValue, ok := defaults[config.Name]; ok {
			value = defaultValue
		}

		result[i] = v1.EnvVar{
			Name:  config.Name,
			Value: fmt.Sprintf("%v", value),
		}
	}

	return result, nil
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
