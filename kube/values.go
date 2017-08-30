package kube

import (
	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// MakeValues returns a ConfigObject with all default values for the Helm chart
func MakeValues(rolesManifest *model.RoleManifest, defaults map[string]string) (helm.ConfigObject, error) {
	env := helm.ConfigObject{}
	for key, value := range model.MakeMapOfVariables(rolesManifest) {
		if !value.Secret || value.Generator == nil || value.Generator.Type != model.GeneratorTypePassword {
			ok, strValue := ConfigValue(value, defaults)
			if !ok {
				strValue = "~"
			}
			env.Add(key, helm.NewConfigScalarWithComment(strValue, value.Description))
		}
	}

	sc := helm.ConfigObject{}
	sc.Add("persistent", helm.NewConfigScalar("persistent"))
	sc.Add("shared", helm.NewConfigScalar("shared"))

	kube := helm.ConfigObject{}
	kube.Add("external_ip", helm.NewConfigScalar("192.168.77.77"))
	kube.Add("storage_class", &sc)

	values := helm.ConfigObject{}
	values.Add("env", &env)
	values.Add("kube", &kube)

	return values, nil
}
