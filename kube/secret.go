package kube

import (
	"encoding/base64"
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"
)

// MakeSecrets creates Secret KubeConfig filled with the
// key/value pairs from the specified map.
func MakeSecrets(secrets model.CVMap, settings ExportSettings) (helm.Node, error) {
	data := helm.NewMapping()
	generated := helm.NewMapping()

	for name, cv := range secrets {
		key := util.ConvertNameToKey(name)
		var value interface{}
		comment := cv.Description

		if settings.CreateHelmChart {
			if cv.Generator == nil {
				if cv.Immutable {
					comment += "\nThis value is immutable and must not be changed once set."
				}
				comment += formattedExample(cv.Example, value)
				required := `{{"" | b64enc | quote}}`
				if cv.Required {
					required = fmt.Sprintf(`{{fail "secrets.%s has not been set"}}`, cv.Name)
				}
				name := ".Values.secrets." + cv.Name
				tmpl := `{{if ne (typeOf %s) "<nil>"}}{{if has (kindOf %s) (list "map" "slice")}}` +
					`{{%s | toJson | b64enc | quote}}{{else}}{{%s | b64enc | quote}}{{end}}{{else}}%s{{end}}`
				value = fmt.Sprintf(tmpl, name, name, name, name, required)
				data.Add(key, helm.NewNode(value, helm.Comment(comment)))
			} else if !cv.Immutable {
				comment += formattedExample(cv.Example, value)
				comment += "\nThis value uses a generated default."
				value = fmt.Sprintf(`{{ default "" .Values.secrets.%s | b64enc | quote }}`, cv.Name)
				generated.Add(key, helm.NewNode(value, helm.Comment(comment)))
			}
			// Immutable secrets with a generator are not user-overridable and only included in the versioned secrets object
		} else {
			ok, value := cv.Value(settings.Defaults)
			if !ok {
				value = ""
			}
			value = base64.StdEncoding.EncodeToString([]byte(value))
			comment += formattedExample(cv.Example, value)
			data.Add(key, helm.NewNode(value, helm.Comment(comment)))
		}
	}
	data.Sort()
	data.Merge(generated.Sort())

	secret := newKubeConfig("v1", "Secret", userSecretsName)
	secret.Add("data", data)

	return secret.Sort(), nil
}
