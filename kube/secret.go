package kube

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/SUSE/fissile/model"
	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

// Secret is a copy of apiv1.Secret
// The Data field is a map of strings, not byte arrays, to avoid base64 encoding.
// The protobug tags, and fields not used here have also been removed.
type Secret struct {
	meta.TypeMeta    `json:",inline"`
	apiv1.ObjectMeta `json:"metadata,omitempty"`
	Data             map[string]string `json:"data,omitempty"`
}

// NewSecret creates a single new, empty K8s Secret
func NewSecret(name string) *Secret {
	secret := &Secret{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: apiv1.ObjectMeta{
			Name: name,
		},
	}
	secret.Data = make(map[string]string)
	return secret
}

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

// MakeSecrets creates an array of new Secrets filled with the
// key/value pairs from the specified map. It further returns a map
// showing which original CV name maps to what secret+key combination.
func MakeSecrets(secrets model.CVMap, defaults map[string]string, createHelmChart bool) ([]*Secret, SecretRefMap, error) {
	var thesecrets []*Secret
	refs := make(map[string]SecretRef)

	max := apiv1.MaxSecretSize
	count := 1
	currentSecret := NewSecret(fmt.Sprintf("secret-%d", count))
	total := 0 // Accumulated size of the values stored in 'currentSecret'

	for key, value := range secrets {
		var strValue string
		if createHelmChart {
			switch {
			case value.Generator == nil || value.Generator.Type != model.GeneratorTypePassword:
				errString := fmt.Sprintf("A valid .Values.env.%s is required", value.Name)
				strValue = fmt.Sprintf(`{{ required "%s" .Values.env.%s | b64enc | quote }}`, errString, value.Name)
			case value.Generator.Type == model.GeneratorTypePassword:
				strValue = "{{ randAlphaNum 32 | b64enc | quote }}"
			default:
				ok, strValue := ConfigValue(value, defaults)
				if !ok {
					return nil, nil, fmt.Errorf("Secret '%s' has no value", key)
				}
				strValue = base64.StdEncoding.EncodeToString([]byte(strValue))
			}
		} else {
			var ok bool
			ok, strValue = ConfigValue(value, defaults)
			if !ok {
				return nil, nil, fmt.Errorf("Secret '%s' has no value", key)
			}
			strValue = base64.StdEncoding.EncodeToString([]byte(strValue))
		}

		bytes := []byte(strValue)
		blen := len(bytes)

		// Bad: This single of our secrets overflows the K8s
		// limit. We cannot store this.
		if blen > max {
			return nil, nil, fmt.Errorf("Secret '%s' overflows K8s limit of %d bytes",
				key, max)
		}
		// Helm charts include template expansion so the size calculation cannot be done here.
		if !createHelmChart {
			// The current entry's value overflows the current K8s
			// secret. Finalize it, and open a new one to store
			// the current entry and anything after.
			if total+blen > max {
				thesecrets = append(thesecrets, currentSecret)
				count++
				currentSecret = NewSecret(fmt.Sprintf("secret-%d", count))
				total = 0
			}
		}

		// (**) From the old transformer we know that "secrets
		// currently need to be lowercase and can only use
		// dashes, not underscores"
		//--
		// name.downcase!.gsub!('_', '-') if var['secret']
		//--
		// Here it is the keys this applies to.

		skey := strings.ToLower(strings.Replace(key, "_", "-", -1))

		currentSecret.Data[skey] = strValue
		refs[key] = SecretRef{
			Secret: currentSecret.ObjectMeta.Name,
			Key:    skey,
		}
		total += blen
	}

	// Save the last K8s secret. Note that it will contain at
	// least one entry, by definition / construction.
	thesecrets = append(thesecrets, currentSecret)

	return thesecrets, refs, nil
}
