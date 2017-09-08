package kube

import (
	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// MakeValues returns a Mapping with all default values for the Helm chart
func MakeValues(rolesManifest *model.RoleManifest, defaults map[string]string) (*helm.Mapping, error) {
	env := helm.NewMapping()
	for name, cv := range model.MakeMapOfVariables(rolesManifest) {
		if !cv.Secret || cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword {
			ok, value := cv.Value(defaults)
			if !ok {
				value = "~"
			}
			env.Add(name, helm.NewScalar(value, helm.Comment(cv.Description)))
		}
	}
	env.Sort()

	sc := helm.NewMapping()
	sc.Add("persistent", helm.NewScalar("persistent"))
	sc.Add("shared", helm.NewScalar("shared"))

	kube := helm.NewMapping()
	kube.Add("external_ip", helm.NewScalar("192.168.77.77"))
	kube.Add("storage_class", sc)

	values := helm.NewMapping()
	values.Add("env", env)
	values.Add("kube", kube)

	return values, nil
}
