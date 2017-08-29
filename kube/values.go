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
			env.Add(key, ConfigScalar{value: strValue, comment: value.Description})
		}
	}

	sc := ConfigObject{}
	sc.Add("persistent", NewConfigScalar("persistent"))
	sc.Add("shared", NewConfigScalar("shared"))

	kube := ConfigObject{}
	kube.Add("external_ip", NewConfigScalar("192.168.77.77"))
	kube.Add("storage_class", sc)

	values := ConfigObject{}
	values.Add("env", env)
	values.Add("kube", kube)

	return values, nil
}
