package kube

import (
	"fmt"

	"github.com/hpcloud/fissile/model"

	meta "k8s.io/client-go/1.5/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/1.5/pkg/api/v1"
	extra "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
)

// NewDeployment creates a Deployment for the given role
func NewDeployment(role *model.Role) (*extra.Deployment, error) {

	podTemplate, podDeps, err := NewPodTemplate(role)
	if err != nil {
		return nil, err
	}

	if len(podDeps) > 0 {
		return nil, fmt.Errorf("Unexpected dependent objects for deployment pod template")
	}

	return &extra.Deployment{
		TypeMeta: meta.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: apiv1.ObjectMeta{
			Name: role.Name,
			Labels: map[string]string{
				RoleNameLabel: role.Name,
			},
		},
		Spec: extra.DeploymentSpec{
			Replicas: &role.Run.Scaling.Min,
			Selector: &extra.LabelSelector{
				MatchLabels: map[string]string{RoleNameLabel: role.Name},
			},
			Template: podTemplate,
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
