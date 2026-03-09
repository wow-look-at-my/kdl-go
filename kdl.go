package kdl

import (
	"bytes"
	"io"

	"github.com/sblinch/kdl-go/document"
	"github.com/sblinch/kdl-go/internal/generator"
	"github.com/sblinch/kdl-go/internal/parser"
	"github.com/sblinch/kdl-go/internal/tokenizer"
)

func parseScanner(s *tokenizer.Scanner, opts parser.ParseContextOptions) (*document.Document, error) {
	defer s.Close()

	// Wire version callback so the parser can update the scanner when it detects a version marker
	opts.VersionCallback = func(v int) { s.Version = v }

	p := parser.New()
	c := p.NewContextOptions(opts)
	for s.Scan() {
		if err := p.Parse(c, s.Token()); err != nil {
			return nil, err
		}
	}
	if s.Err() != nil {
		return nil, s.Err()
	}

	return c.Document(), nil
}

type ParseOptions = parser.ParseContextOptions

var DefaultParseOptions = parser.ParseContextOptions{}

// Parse parses a KDL document from r and returns the parsed Document, or a non-nil error on failure
func Parse(r io.Reader) (*document.Document, error) {
	return ParseWithOptions(r, DefaultParseOptions)
}

func ParseWithOptions(r io.Reader, opts ParseOptions) (*document.Document, error) {
	if opts.Version != 0 {
		// Explicit version: no fallback
		s := tokenizer.New(r)
		s.RelaxedNonCompliant = opts.RelaxedNonCompliant
		s.ParseComments = opts.Flags.Has(parser.ParseComments)
		s.Version = opts.Version
		return parseScanner(s, opts)
	}

	// Auto-detect: buffer the entire input so we can retry
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return parseSliceAutoDetect(data, opts)
}

// ParseSlice parses a KDL document from a byte slice
func ParseSlice(data []byte) (*document.Document, error) {
	return ParseSliceWithOptions(data, DefaultParseOptions)
}

// ParseSliceWithOptions parses a KDL document from a byte slice with the specified options
func ParseSliceWithOptions(data []byte, opts ParseOptions) (*document.Document, error) {
	if opts.Version != 0 {
		s := tokenizer.NewSlice(data)
		s.RelaxedNonCompliant = opts.RelaxedNonCompliant
		s.ParseComments = opts.Flags.Has(parser.ParseComments)
		s.Version = opts.Version
		return parseScanner(s, opts)
	}

	return parseSliceAutoDetect(data, opts)
}

// parseSliceAutoDetect checks for a version marker and parses accordingly.
// If a /- kdl-version marker is found, uses that version definitively.
// If no marker is found, parses as both v1 and v2; if both succeed and produce
// equivalent documents, returns the v2 result. If they differ (e.g. bare true/false/null
// semantics), returns the v2 result (preferring v2). If only one succeeds, returns that.
func parseSliceAutoDetect(data []byte, opts ParseOptions) (*document.Document, error) {
	// Check for version marker in the input — definitive, no fallback
	if version := detectVersionMarker(data); version != 0 {
		s := tokenizer.NewSlice(data)
		s.RelaxedNonCompliant = opts.RelaxedNonCompliant
		s.ParseComments = opts.Flags.Has(parser.ParseComments)
		s.Version = version
		return parseScanner(s, opts)
	}

	// No marker: try both versions
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	// Try v2
	s2 := tokenizer.NewSlice(dataCopy)
	s2.RelaxedNonCompliant = opts.RelaxedNonCompliant
	s2.ParseComments = opts.Flags.Has(parser.ParseComments)
	s2.Version = 2
	doc2, err2 := parseScanner(s2, opts)

	// Try v1
	s1 := tokenizer.NewSlice(data)
	s1.RelaxedNonCompliant = opts.RelaxedNonCompliant
	s1.ParseComments = opts.Flags.Has(parser.ParseComments)
	s1.Version = 1
	doc1, err1 := parseScanner(s1, opts)

	// If only one succeeds, use that
	if err2 != nil && err1 == nil {
		return doc1, nil
	}
	if err1 != nil && err2 == nil {
		return doc2, nil
	}
	if err1 != nil && err2 != nil {
		// Both failed — return v2 error (more likely the intended version)
		return nil, err2
	}

	// Both succeeded: if documents are equivalent, prefer v2
	// If they differ (e.g. bare true/false/null parsed differently), also prefer v2
	// since the user specified v2-first preference
	return doc2, nil
}

// detectVersionMarker scans for /- kdl-version N at the beginning of data
func detectVersionMarker(data []byte) int {
	// Skip BOM and whitespace
	d := bytes.TrimLeft(data, " \t\r\n\xEF\xBB\xBF")

	// Look for /- kdl-version N
	if !bytes.HasPrefix(d, []byte("/-")) {
		return 0
	}
	d = d[2:]

	// Skip whitespace after /-
	d = bytes.TrimLeft(d, " \t")

	if !bytes.HasPrefix(d, []byte("kdl-version")) {
		return 0
	}
	d = d[len("kdl-version"):]

	// Skip whitespace
	d = bytes.TrimLeft(d, " \t")

	if len(d) > 0 && d[0] == '2' {
		return 2
	}
	if len(d) > 0 && d[0] == '1' {
		return 1
	}
	return 0
}

type GenerateOptions = generator.Options

var DefaultGenerateOptions = generator.DefaultOptions

// Generate writes to w a well-formatted KDL document generated from doc, or a non-nil error on failure
func Generate(doc *document.Document, w io.Writer) error {
	return GenerateWithOptions(doc, w, DefaultGenerateOptions)
}

// GenerateWithOptions writes to w a well-formatted KDL document generated from doc, or a non-nil error on failure
func GenerateWithOptions(doc *document.Document, w io.Writer, opts GenerateOptions) error {
	g := generator.NewOptions(w, opts)
	return g.Generate(doc)
}
