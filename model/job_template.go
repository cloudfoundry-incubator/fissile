package model

import (
	"fmt"
	"io"
	"strings"

	assets "github.com/hpcloud/fissile/scripts/templates"
	"github.com/hpcloud/fissile/templates"

	"github.com/benbjohnson/ego"
)

// JobTemplate represents a BOSH job template
type JobTemplate struct {
	SourcePath      string
	DestinationPath string
	Job             *Job
	Content         string
}

const (
	// TextBlock is the text section of a BOSH template
	TextBlock = "text"
	// PrintBlock is the print section of a BOSH template
	PrintBlock = "print"
	// CodeBlock is the code section of a BOSH template
	CodeBlock = "code"
)

// TemplateBlock is a BOSH template block
type TemplateBlock struct {
	Type  string
	Block string
}

// Transform will convert the template
func (b *TemplateBlock) Transform() (string, error) {
	var transformation []byte
	var err error

	switch {
	case b.Type == PrintBlock:
		transformation, err = assets.Asset("scripts/templates/transformations.yml")
	case b.Type == CodeBlock:
		transformation, err = assets.Asset("scripts/templates/transformations_code.yml")
	default:
		return b.Block, nil
	}

	if err != nil {
		return "", fmt.Errorf("Error loading script asset. This is probably a bug: %s", err.Error())
	}

	transformer, err := templates.NewTransformer(string(transformation))
	if err != nil {
		return "", fmt.Errorf("Error loading transformation yaml. This is probably a bug: %s", err.Error())
	}

	return transformer.Transform(b.Block)
}

// GetErbBlocks will find the erb blocks within the job template
func (j *JobTemplate) GetErbBlocks() ([]*TemplateBlock, error) {
	result := []*TemplateBlock{}

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
			result = append(result, &TemplateBlock{
				Type:  TextBlock,
				Block: b.(*ego.TextBlock).Content,
			})
		case *ego.CodeBlock:
			result = append(result, &TemplateBlock{
				Type:  CodeBlock,
				Block: b.(*ego.CodeBlock).Content,
			})
		case *ego.PrintBlock:
			result = append(result, &TemplateBlock{
				Type:  PrintBlock,
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
