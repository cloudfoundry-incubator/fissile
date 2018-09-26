package kube

import (
	"encoding/base64"
	"fmt"
	"strings"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
)

// MakeSecrets creates Secret KubeConfig filled with the
// key/value pairs from the specified map.
func MakeSecrets(secrets model.CVMap, settings ExportSettings) (helm.Node, error) {
	data := helm.NewMapping()
	generated := helm.NewMapping()

	for name, cv := range secrets {
		key := util.ConvertNameToKey(name)
		var value interface{}
		comment := cv.CVOptions.Description

		if settings.CreateHelmChart {
			// cv.Generator == nil
			if cv.Type == "" && independentSecret(cv.Name) {
				if cv.CVOptions.Immutable {
					comment += "\nThis value is immutable and must not be changed once set."
				}
				comment += formattedExample(cv.CVOptions.Example, value)
				required := `{{"" | b64enc | quote}}`
				if cv.CVOptions.Required {
					required = fmt.Sprintf(`{{fail "secrets.%s has not been set"}}`, cv.Name)
				}
				name := ".Values.secrets." + cv.Name
				tmpl := `{{if ne (typeOf %s) "<nil>"}}{{if has (kindOf %s) (list "map" "slice")}}` +
					`{{%s | toJson | b64enc | quote}}{{else}}{{%s | b64enc | quote}}{{end}}{{else}}%s{{end}}`
				value = fmt.Sprintf(tmpl, name, name, name, name, required)
				data.Add(key, helm.NewNode(value, helm.Comment(comment)))
			} else if !cv.CVOptions.Immutable {
				comment += formattedExample(cv.CVOptions.Example, value)
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
			comment += formattedExample(cv.CVOptions.Example, value)
			data.Add(key, helm.NewNode(value, helm.Comment(comment)))
		}
	}
	data.Sort()
	data.Merge(generated.Sort())

	secret := newKubeConfig("v1", "Secret", userSecretsName)
	secret.Add("data", data)

	return secret.Sort(), nil
}

func independentSecret(name string) bool {
	return !strings.HasSuffix(name, "_KEY") && !strings.HasSuffix(name, "_FINGERPRINT")
}
