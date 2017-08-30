package kube

import (
	"github.com/SUSE/fissile/model"
)

// MakeValues returns a ConfigObject with all default values for the Helm chart
func MakeValues(rolesManifest *model.RoleManifest, defaults map[string]string) (ConfigObject, error) {
	env := ConfigObject{}
	for key, value := range model.MakeMapOfVariables(rolesManifest) {
		if !value.Secret || value.Generator == nil || value.Generator.Type != model.GeneratorTypePassword {
			ok, strValue := ConfigValue(value, defaults)
			if !ok {
				strValue = "~"
			}
			env.add(key, &ConfigScalar{value: strValue, comment: value.Description})
		}
	}

	sc := ConfigObject{}
	sc.add("persistent", NewConfigScalar("persistent"))
	sc.add("shared", NewConfigScalar("shared"))

	kube := ConfigObject{}
	kube.add("external_ip", NewConfigScalar("192.168.77.77"))
	kube.add("storage_class", &sc)

	values := ConfigObject{}
	values.add("env", &env)
	values.add("kube", &kube)

	return values, nil
}
