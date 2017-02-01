package kube

import (
	"github.com/hpcloud/fissile/model"

	meta "k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/api/v1"
	extra "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
)

func NewDeployment(role *model.Role) *extra.Deployment {

	replicas := int32(role.Run.Scaling.Min)

	return &extra.Deployment{
		TypeMeta: meta.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: role.Name,
			Labels: map[string]string{
				RoleNameLabel: role.Name,
			},
		},
		Spec: extra.DeploymentSpec{
			Replicas: &replicas,
			Selector: &extra.LabelSelector{
				MatchLabels: map[string]string{RoleNameLabel: role.Name},
			},
			Template: NewPodTemplate(role),
		},
	}
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
