package templates

import (
	"bytes"
	"regexp"
	"text/template"

	"gopkg.in/yaml.v2"
)

// Match represents a matcher
type Match struct {
	Match           string  `yaml:"match"`
	Output          string  `yaml:"output"`
	Transformations []Match `yaml:"transformations,flow,omitempty"`
}

// Transformer represents the transformers of a package
type Transformer struct {
	Transformations []Match `yaml:"transformations,flow"`
}

// NewTransformer will create an instance of the transformer from the YAML
func NewTransformer(spec string) (*Transformer, error) {
	transformer := &Transformer{}

	if err := yaml.Unmarshal([]byte(spec), transformer); err != nil {
		return nil, err
	}

	return transformer, nil
}

// Transform performs the transformation
func (t *Transformer) Transform(input string) (string, error) {
	return getFirstSuccesfulTransform(input, t.Transformations)
}

func (t *Match) transform(input string) (string, error) {
	r := regexp.MustCompile(t.Match)
	context := r.FindAllString(input, -1)

	if len(context) == 0 {
		return "", nil
	}

	for idx, match := range context {
		transformed, err := getFirstSuccesfulTransform(match, t.Transformations)
		if err != nil {
			return "", err
		}

		if transformed != "" {
			context[idx] = transformed
		}
	}

	matchTemplate := template.New("template-transformer")
	matchTemplate.Delims("((", "))")
	matchTemplate, err := matchTemplate.Parse(t.Output)
	if err != nil {
		return "", err
	}

	var output bytes.Buffer
	err = matchTemplate.Execute(&output, context)
	if err != nil {
		return "", err
	}

	return output.String(), nil
}

func getFirstSuccesfulTransform(input string, matches []Match) (string, error) {
	for _, match := range matches {
		output, err := match.transform(input)

		if err != nil {
			return "", err
		}

		if output != "" {
			return output, nil
		}
	}

	return "", nil
}
