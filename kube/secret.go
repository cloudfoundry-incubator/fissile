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
	for name, cv := range secrets {
		var value string

		key := util.ConvertNameToKey(name)

		if settings.CreateHelmChart {
			if settings.UseSecretsGenerator && cv.Generator != nil {
				// it will be created during runtime
				continue
			}
			if cv.Generator != nil && cv.Generator.Type == model.GeneratorTypePassword {
				value = "{{ randAlphaNum 32 | b64enc | quote }}"
			} else {
				errString := fmt.Sprintf("%s configuration missing", cv.Name)
				value = fmt.Sprintf(`{{ required "%s" .Values.env.%s | b64enc | quote }}`, errString, cv.Name)
			}
		} else {
			ok, value := cv.Value(settings.Defaults)
			if !ok {
				value = ""
			}
			value = base64.StdEncoding.EncodeToString([]byte(value))
		}

		data.Add(key, helm.NewNode(value, helm.Comment(cv.Description)))
	}
	data.Sort()

	secretName := "secret"
	if settings.UseSecretsGenerator {
		secretName = "secret-update"
		if settings.CreateHelmChart {
			secretName = "secret-update-{{ .Release.Revision }}"
		}
	}
	secret := newKubeConfig("v1", "Secret", secretName)
	secret.Add("data", data)

	return secret.Sort(), nil
}
