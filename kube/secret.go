package kube

import (
	"fmt"
	//	"strconv"
	"strings"

	"github.com/SUSE/fissile/model"
	meta "k8s.io/client-go/pkg/api/unversioned"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

// NewSecret creates a single new, empty K8s Secret
func NewSecret(name string) *apiv1.Secret {
	secret := &apiv1.Secret{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: apiv1.ObjectMeta{
			Name: name,
		},
	}
	secret.Data = make(map[string][]byte)
	return secret
}

// Ref is an entry in the RefMap
type Ref struct {
	Secret string
	Key    string
}

// RefMap maps the names of secret CVs to the combination of secret
// and key used to store their value. Note that the key has to be
// stored, because of the transformation at (**). Ok, not truly, but
// then we would have to replicate the transform at the place where
// the mapping is used. I prefer to do it only once.
type RefMap map[string]Ref

// MakeSecrets creates an array of new Secrets filled with the
// key/value pairs from the specified map. It further returns a map
// showing which original CV name maps to what secret+key combination.
func MakeSecrets(secrets model.CVMap, defaults map[string]string) ([]*apiv1.Secret, RefMap, error) {
	var thesecrets []*apiv1.Secret
	refs := make(map[string]Ref)

	max := apiv1.MaxSecretSize
	count := 1
	current := NewSecret(fmt.Sprintf("secret-%d", count))
	total := 0 // Accumulated size of the values stored in 'current'

	for key, value := range secrets {
		ok, strValue := configValue(value, defaults)
		if !ok {
			return nil, nil, fmt.Errorf("Secret '%s' has no value", key)
		}

		bytes := []byte(strValue)
		blen := len(bytes)

		// Bad: This single of our secrets overflows the K8s
		// limit. We cannot store this.
		if blen > max {
			return nil, nil, fmt.Errorf("Secret '%s' overflows K8s limit of %d bytes",
				key, max)
		}
		// The current entry's value overflows the current K8s
		// secret. Finalize it, and open a new one to store
		// the current entry and anything after.
		if total+blen > max {
			thesecrets = append(thesecrets, current)
			count = count + 1
			current = NewSecret(fmt.Sprintf("secret-%d", count))
			total = 0
		}

		// (**) From the old transformer we know that "secrets
		// currently need to be lowercase and can only use
		// dashes, not underscores"
		//--
		// name.downcase!.gsub!('_', '-') if var['secret']
		//--
		// Here it is the keys this applies to.

		skey := strings.ToLower(strings.Replace(key, "_", "-", -1))

		current.Data[skey] = bytes
		refs[key] = Ref{
			Secret: current.ObjectMeta.Name,
			Key:    skey,
		}
		total = total + blen
	}

	// Save the last K8s secret. Note that it will contain at
	// least one entry, by definition / construction.
	thesecrets = append(thesecrets, current)

	return thesecrets, refs, nil
}
