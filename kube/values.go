package kube

import (
	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// MakeValues returns a Mapping with all default values for the Helm chart
func MakeValues(roleManifest *model.RoleManifest, defaults map[string]string) (helm.Node, error) {
	env := helm.NewEmptyMapping()
	for name, cv := range model.MakeMapOfVariables(roleManifest) {
		if !cv.Secret || cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword {
			ok, value := cv.Value(defaults)
			if !ok {
				value = "~"
			}
			env.AddNode(name, helm.NewScalar(value, helm.Comment(cv.Description)))
		}
	}
	env.Sort()

	storageClass := helm.NewEmptyMapping()
	storageClass.AddNode("persistent", helm.NewScalar("persistent"))
	storageClass.AddNode("shared", helm.NewScalar("shared"))

	kube := helm.NewEmptyMapping()
	kube.AddNode("external_ip", helm.NewScalar("192.168.77.77"))
	kube.AddNode("storage_class", storageClass)

	values := helm.NewEmptyMapping()
	values.AddNode("env", env)
	values.AddNode("kube", kube)

	return values, nil
}
