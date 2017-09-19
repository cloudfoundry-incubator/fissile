package kube

import (
	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// MakeValues returns a Mapping with all default values for the Helm chart
func MakeValues(rolesManifest *model.RoleManifest, defaults map[string]string) (helm.Node, error) {
	env := helm.NewEmptyMapping()
	for name, cv := range model.MakeMapOfVariables(rolesManifest) {
		if !cv.Secret || cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword {
			ok, value := cv.Value(defaults)
			if !ok {
				value = "~"
			}
			env.AddNode(name, helm.NewScalar(value, helm.Comment(cv.Description)))
		}
	}
	env.Sort()

	storqageClass := helm.NewEmptyMapping()
	storqageClass.AddNode("persistent", helm.NewScalar("persistent"))
	storqageClass.AddNode("shared", helm.NewScalar("shared"))

	kube := helm.NewEmptyMapping()
	kube.AddNode("external_ip", helm.NewScalar("192.168.77.77"))
	kube.AddNode("storage_class", storqageClass)

	values := helm.NewEmptyMapping()
	values.AddNode("env", env)
	values.AddNode("kube", kube)

	return values, nil
}
