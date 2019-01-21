package kube

import (
	"regexp"

	"code.cloudfoundry.org/fissile/helm"
)

// GetHelmTemplateHelpers returns the helm templates needed throughout the code.
func GetHelmTemplateHelpers() []helm.Node {
	sanitizeNameHelper := `
		{{ define "fissile.SanitizeName" }}
			{{ if lt (len .) 1 }}{{ fail "No name given for node" }}{{ end }}
			{{ if gt (len .) 63 }}
				{{ . | trunc 54 }}-{{ . | sha256sum | trunc 8 }}
			{{ else }}
				{{ . }}
			{{ end }}
		{{ end }}`
	return []helm.Node{
		helm.NewNode(
			regexp.MustCompile(`(^|\}\})\s+(\{\{|$)`).
				ReplaceAllString(sanitizeNameHelper, "${1}${2}"),
			helm.Comment(`
				fissile.SanitizeName returns the given parameter, up to 63 characters long.
				This should be called as {{ template "fissile.SanitizeName" "foo" }}
				`)),
	}
}
