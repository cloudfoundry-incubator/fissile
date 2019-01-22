package kube

import (
	"fmt"
	"strings"
	"testing"

	"code.cloudfoundry.org/fissile/helm"
	"github.com/stretchr/testify/assert"
)

func TestGetHelmTemplateHelpers(t *testing.T) {
	t.Run("fissile.SanitizeName", func(t *testing.T) {
		for _, testcase := range []struct {
			length   int
			expected string
		}{
			{
				length:   63,
				expected: strings.Repeat("a", 63),
			},
			{
				length:   64,
				expected: strings.Repeat("a", 54) + "-ffe054fe",
			},
		} {
			func(length int, expected string) {
				t.Run(fmt.Sprintf("name of length %d", length), func(t *testing.T) {
					t.Parallel()
					name := strings.Repeat("a", length)
					node := helm.NewNode(fmt.Sprintf(`{{ template "fissile.SanitizeName" %q }}`, name))
					rendered, err := RoundtripNode(node, nil)
					assert.True(t, len(rendered.(string)) < 64, "sanitized name is too long")
					if assert.NoError(t, err) {
						assert.Equal(t, expected, rendered.(string))
					}
				})
			}(testcase.length, testcase.expected)
		}
	})
}
