package helm

import (
	"github.com/SUSE/fissile/model"
)

// MakeValues returns an Object with all default values for the Helm chart
func MakeValues(rolesManifest *model.RoleManifest, defaults map[string]string) (*Object, error) {
	env := NewObject()
	for name, cv := range model.MakeMapOfVariables(rolesManifest) {
		if !cv.Secret || cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword {
			ok, value := cv.Value(defaults)
			if !ok {
				value = "~"
			}
			env.Add(name, NewScalar(value, Comment(cv.Description)))
		}
	}

	sc := NewObject()
	sc.Add("persistent", NewScalar("persistent"))
	sc.Add("shared", NewScalar("shared"))

	kube := NewObject()
	kube.Add("external_ip", NewScalar("192.168.77.77"))
	kube.Add("storage_class", sc)

	values := NewObject()
	values.Add("env", env)
	values.Add("kube", kube)

	return values, nil
}
