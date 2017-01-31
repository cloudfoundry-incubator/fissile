package kube

import (
	"bytes"

	"k8s.io/client-go/1.5/pkg/api"
	meta "k8s.io/client-go/1.5/pkg/api/unversioned"
)

func WriteConfig(namespace string) (string, error) {

	r := &api.Namespace{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: api.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"what": "hcf",
			},
		},
	}

	serializer, ok := api.Codecs.SerializerForFileExtension("yaml")
	if !ok {
		// There's a problem with the code, if we can't find the yaml serializer
		panic("Can't find the kubernetes yaml serializer")
	}

	buf := new(bytes.Buffer)
	err := serializer.Encode(r, buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
