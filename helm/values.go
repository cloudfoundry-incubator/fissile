package helm

import (
	"github.com/SUSE/fissile/model"
)

// MakeValues returns an Object with all default values for the Helm chart
func MakeValues(rolesManifest *model.RoleManifest, defaults map[string]string) (Object, error) {
	env := Object{}
	for name, cv := range model.MakeMapOfVariables(rolesManifest) {
		if !cv.Secret || cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword {
			ok, value := cv.Value(defaults)
			if !ok {
				value = "~"
			}
			env.Add(name, NewScalarWithComment(value, cv.Description))
		}
	}

	sc := Object{}
	sc.Add("persistent", NewScalar("persistent"))
	sc.Add("shared", NewScalar("shared"))

	kube := Object{}
	kube.Add("external_ip", NewScalar("192.168.77.77"))
	kube.Add("storage_class", &sc)

	values := Object{}
	values.Add("env", &env)
	values.Add("kube", &kube)

	return values, nil
}
