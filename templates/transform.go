package templates

import (
	"bytes"
	"gopkg.in/yaml.v2"
	"regexp"
	"text/template"
)

type Match struct {
	Match           string  `yaml:"match"`
	Output          string  `yaml:"output"`
	Transformations []Match `yaml:"transformations,flow,omitempty"`
}

type Transformer struct {
	Transformations []Match `yaml:"transformations,flow"`
}

func NewTransformer(spec string) (*Transformer, error) {
	transformer := &Transformer{}

	if err := yaml.Unmarshal([]byte(spec), transformer); err != nil {
		return nil, err
	}

	return transformer, nil
}

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

	matchTemplate := template.New("t")
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
