package kube

import (
	"io"

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

// WriteYamlConfig writes the YAML serialized configuration of a k8s object to
// a specified writer
func WriteYamlConfig(kubeObject runtime.Object, writer io.Writer) error {
	Scheme := runtime.NewScheme()
	if err := api.AddToScheme(Scheme); err != nil {
		// Programmer error, detect immediately
		panic(err)
	}
	if err := v1.AddToScheme(Scheme); err != nil {
		// Programmer error, detect immediately
		panic(err)
	}

	if _, err := writer.Write([]byte("---\n")); err != nil {
		return err
	}

	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, Scheme, Scheme)
	if err := serializer.Encode(kubeObject, writer); err != nil {
		return err
	}

	return nil
}
