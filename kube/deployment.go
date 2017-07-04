package kube

import (
	"github.com/SUSE/fissile/model"

	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	extra "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// NewDeployment creates a Deployment for the given role, and its attached services
func NewDeployment(role *model.Role, settings *ExportSettings) (*extra.Deployment, *apiv1.List, error) {

	podTemplate, err := NewPodTemplate(role, settings)
	if err != nil {
		return nil, nil, err
	}

	svc, err := NewClusterIPServiceList(role, false)
	if err != nil {
		return nil, nil, err
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
			Selector: &meta.LabelSelector{
				MatchLabels: map[string]string{RoleNameLabel: role.Name},
			},
			Template: podTemplate,
		},
	}, svc, nil
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
