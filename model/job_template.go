package model

import (
	"fmt"
	"io"
	"strings"

	assets "github.com/hpcloud/fissile/scripts/templates"
	"github.com/hpcloud/fissile/templates"

	"github.com/benbjohnson/ego"
)

type JobTemplate struct {
	SourcePath      string
	DestinationPath string
	Job             *Job
	Content         string
}

const (
	textBlock  = "text"
	printBlock = "print"
	codeBlock  = "code"
)

type templateBlock struct {
	Type  string
	Block string
}

func (b *templateBlock) Transform() (string, error) {
	transformation, err := assets.Asset("scripts/templates/transformations.yml")
	if err != nil {
		return "", fmt.Errorf("Error loading script asset. This is probably a bug: %s", err.Error())
	}

	transformer, err := templates.NewTransformer(string(transformation))
	if err != nil {
		return "", fmt.Errorf("Error loading transformation yaml. This is probably a bug: %s", err.Error())
	}

	return transformer.Transform(b.Block)
}

func (j *JobTemplate) GetErbBlocks() ([]*templateBlock, error) {
	result := []*templateBlock{}

	s := ego.NewScanner(strings.NewReader(j.Content), "")

	b, err := s.Scan()

	if err != nil {
		return nil, err
	}

	for b != nil && err == nil {
		switch v := b.(type) {
		default:
			return nil, fmt.Errorf("Unexpected block type %T in template %s for job %s", v, j.SourcePath, j.Job.Name)
		case *ego.TextBlock:
			result = append(result, &templateBlock{
				Type:  textBlock,
				Block: b.(*ego.TextBlock).Content,
			})
		case *ego.CodeBlock:
			result = append(result, &templateBlock{
				Type:  codeBlock,
				Block: b.(*ego.CodeBlock).Content,
			})
		case *ego.PrintBlock:
			result = append(result, &templateBlock{
				Type:  printBlock,
				Block: b.(*ego.PrintBlock).Content,
			})
		}

		b, err = s.Scan()
	}

	if err != nil && err != io.EOF {
		return nil, err
	}

	return result, nil
}
