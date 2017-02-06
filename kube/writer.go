package kube

import (
	"bytes"

	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/runtime/serializer/json"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	// RoleNameLabel is a thing
	RoleNameLabel = "skiff-role-name"
	// VolumeStorageClassAnnotation is the annotation label for storage/v1beta1/StorageClass
	VolumeStorageClassAnnotation = "volume.beta.kubernetes.io/storage-class"
)

// GetYamlConfig returns the YAML serialized configuration of a k8s object
func GetYamlConfig(kubeObject runtime.Object) (string, error) {
	Scheme := runtime.NewScheme()
	if err := api.AddToScheme(Scheme); err != nil {
		// Programmer error, detect immediately
		panic(err)
	}
	if err := v1.AddToScheme(Scheme); err != nil {
		// Programmer error, detect immediately
		panic(err)
	}

	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, Scheme, Scheme)

	buf := new(bytes.Buffer)
	err := serializer.Encode(kubeObject, buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
