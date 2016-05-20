package fake_extractor

type FakeExtractor struct {
	extractInput struct {
		src  string
		dest string
	}
	extractOutput struct {
		err error
	}
}

func (extractor *FakeExtractor) Extract(src, dest string) error {
	extractor.extractInput.src = src
	extractor.extractInput.dest = dest
	return extractor.extractOutput.err
}

func (extractor *FakeExtractor) ExtractInput() (src, dest string) {
	return extractor.extractInput.src, extractor.extractInput.dest
}
func (extractor *FakeExtractor) SetExtractOutput(err error) {
	extractor.extractOutput.err = err
}
