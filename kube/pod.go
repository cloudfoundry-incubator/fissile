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
func NewPodTemplate(role *model.Role, repository string, defaults map[string]string) (v1.PodTemplateSpec, []runtime.Object, error) {

	vars, err := getEnvVars(role, defaults)
	if err != nil {
		return v1.PodTemplateSpec{}, nil, err
	}

	imageName := builder.GetRoleDevImageName(repository, role, role.GetRoleDevVersion())

	volumes, volumeClaims := getVolumes(role)

	deps := make([]runtime.Object, 0, len(volumeClaims))
	for _, volumeClaim := range volumeClaims {
		deps = append(deps, volumeClaim)
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
			Volumes:       volumes,
			RestartPolicy: v1.RestartPolicyAlways,
			DNSPolicy:     v1.DNSClusterFirst,
		},
	}, deps, nil
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

func getVolumeMounts(role *model.Role) []v1.VolumeMount {
	result := make([]v1.VolumeMount, len(role.Run.PersistentVolumes)+len(role.Run.SharedVolumes))

	for i, volume := range role.Run.PersistentVolumes {
		result[i] = v1.VolumeMount{
			Name:      volume.Tag,
			MountPath: volume.Path,
			ReadOnly:  false,
		}
	}

	for i, volume := range role.Run.SharedVolumes {
		result[len(role.Run.PersistentVolumes)+i] = v1.VolumeMount{
			Name:      volume.Tag,
			MountPath: volume.Path,
			ReadOnly:  false,
		}
	}

	return result
}

func getVolumes(role *model.Role) []v1.Volume {
	result := make([]v1.Volume, len(role.Run.PersistentVolumes)+len(role.Run.SharedVolumes))

	for i, volume := range role.Run.PersistentVolumes {
		result[i] = v1.Volume{
			Name: volume.Tag,
			// TODO needs a volume source
		}
	}

	for i, volume := range role.Run.SharedVolumes {
		result[len(role.Run.PersistentVolumes)+i] = v1.Volume{
			Name: volume.Tag,
		}
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
