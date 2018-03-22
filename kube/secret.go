package kube

import (
	"encoding/base64"
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"
)

// MakeSecrets creates Secret KubeConfig filled with the
// key/value pairs from the specified map. It further returns a map
// showing which original CV name maps to what secret+key combination.
func MakeSecrets(secrets model.CVMap, settings ExportSettings) (helm.Node, error) {
	data := helm.NewMapping()
	generated := helm.NewMapping()

	for name, cv := range secrets {
		var value interface{}

		key := util.ConvertNameToKey(name)
		comment := cv.Description

		if settings.CreateHelmChart {
			if !settings.UseSecretsGenerator && cv.Generator != nil && cv.Generator.Type == model.GeneratorTypePassword {
				value = "{{ randAlphaNum 32 | b64enc | quote }}"
			} else {
				if settings.UseSecretsGenerator && cv.Generator != nil {
					comment += "\nThis value uses a generated default."
					if cv.Immutable {
						comment += "\nIt is also immutable and must not be changed once set."
					}
					value = fmt.Sprintf(`{{ default "" .Values.secrets.%s | b64enc | quote }}`, cv.Name)
					generated.Add(key, helm.NewNode(value, helm.Comment(comment)))
					continue
				} else {
					errString := fmt.Sprintf("secrets.%s has not been set", cv.Name)
					value = fmt.Sprintf(`{{ required "%s" .Values.secrets.%s | b64enc | quote }}`, errString, cv.Name)
					if cv.Immutable {
						comment += "\nThis value is immutable and must not be changed once set."
					}

				}
			}
		} else {
			ok, value := cv.Value(settings.Defaults)
			if !ok {
				value = ""
			}
			value = base64.StdEncoding.EncodeToString([]byte(value))
		}

		data.Add(key, helm.NewNode(value, helm.Comment(comment)))
	}
	data.Sort()
	data.Merge(generated.Sort())

	secret := newKubeConfig("v1", "Secret", "secrets")
	secret.Add("data", data)

	return secret.Sort(), nil
}
