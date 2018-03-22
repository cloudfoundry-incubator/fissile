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
		key := util.ConvertNameToKey(name)
		var value interface{}
		comment := cv.Description

		if settings.CreateHelmChart {
			if cv.Generator == nil {
				errString := fmt.Sprintf("secrets.%s has not been set", cv.Name)
				value = fmt.Sprintf(`{{ required "%s" .Values.secrets.%s | b64enc | quote }}`, errString, cv.Name)
				if cv.Immutable {
					comment += "\nThis value is immutable and must not be changed once set."
				}
				data.Add(key, helm.NewNode(value, helm.Comment(comment)))
			} else if !cv.Immutable {
				comment += "\nThis value uses a generated default."
				value = fmt.Sprintf(`{{ default "" .Values.secrets.%s | b64enc | quote }}`, cv.Name)
				generated.Add(key, helm.NewNode(value, helm.Comment(comment)))
			}
		} else {
			ok, value := cv.Value(settings.Defaults)
			if !ok {
				value = ""
			}
			value = base64.StdEncoding.EncodeToString([]byte(value))
			data.Add(key, helm.NewNode(value, helm.Comment(comment)))
		}
	}
	data.Sort()
	data.Merge(generated.Sort())

	secret := newKubeConfig("v1", "Secret", "secrets")
	secret.Add("data", data)

	return secret.Sort(), nil
}
