package kube

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/SUSE/fissile/helm"
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
func MakeSecrets(secrets model.CVMap, defaults map[string]string, createHelmChart bool) (*helm.Object, SecretRefMap, error) {
	refs := make(map[string]SecretRef)

	data := helm.NewObject()
	for name, cv := range secrets {
		var value string
		if createHelmChart {
			switch {
			case cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword:
				errString := fmt.Sprintf("%s configuration missing", cv.Name)
				value = fmt.Sprintf(`{{ required "%s" .Values.env.%s | b64enc | quote }}`, errString, cv.Name)
			case cv.Generator.Type == model.GeneratorTypePassword:
				value = "{{ randAlphaNum 32 | b64enc | quote }}"
			}
		} else {
			ok, value := cv.Value(defaults)
			if !ok {
				return nil, nil, fmt.Errorf("Secret '%s' has no value", name)
			}
			value = base64.StdEncoding.EncodeToString([]byte(value))
		}

		// (**) "secrets need to be lowercase and can only use dashes, not underscores"
		key := strings.ToLower(strings.Replace(name, "_", "-", -1))

		data.Add(key, helm.NewScalar(value, helm.Comment(cv.Description)))
		refs[name] = SecretRef{
			Secret: "secret",
			Key:    key,
		}
	}
	data.Sort()

	secret := newKubeConfig("Secret", "secret")
	secret.Add("data", data)

	return secret, refs, nil
}
