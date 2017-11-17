package kube

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
