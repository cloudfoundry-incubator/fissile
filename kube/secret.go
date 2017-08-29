package kube

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/SUSE/fissile/model"
)

// SecretRef is an entry in the SecretRefMap
type SecretRef struct {
	Secret string
	Key    string
}

// SecretRefMap maps the names of secret CVs to the combination of
// secret and key used to store their value. Note that the key has to
// be stored, because of the transformation at (**). Ok, not truly,
// but then we would have to replicate the transform at the place
// where the mapping is used. I prefer to do it only once.
type SecretRefMap map[string]SecretRef

// MakeSecrets creates Secret KubeConfig filled with the
// key/value pairs from the specified map. It further returns a map
// showing which original CV name maps to what secret+key combination.
func MakeSecrets(secrets model.CVMap, defaults map[string]string, createHelmChart bool) (*ConfigObject, SecretRefMap, error) {
	refs := make(map[string]SecretRef)

	data := ConfigObject{}
	for key, value := range secrets {
		var strValue string
		if createHelmChart {
			switch {
			case value.Generator == nil || value.Generator.Type != model.GeneratorTypePassword:
				errString := fmt.Sprintf("%s configuration missing", value.Name)
				strValue = fmt.Sprintf(`{{ required "%s" .Values.env.%s | b64enc | quote }}`, errString, value.Name)
			case value.Generator.Type == model.GeneratorTypePassword:
				strValue = "{{ randAlphaNum 32 | b64enc | quote }}"
			}
		} else {
			ok, strValue := ConfigValue(value, defaults)
			if !ok {
				return nil, nil, fmt.Errorf("Secret '%s' has no value", key)
			}
			strValue = base64.StdEncoding.EncodeToString([]byte(strValue))
		}

		// (**) "secrets need to be lowercase and can only use dashes, not underscores"
		skey := strings.ToLower(strings.Replace(key, "_", "-", -1))

		data.Add(skey, ConfigScalar{value: strValue, comment: value.Description})
		refs[key] = SecretRef{
			Secret: "secret",
			Key:    skey,
		}
	}

	secret := NewKubeConfig("Secret", "secret")
	secret.Add("data", data)

	return &secret, refs, nil
}
