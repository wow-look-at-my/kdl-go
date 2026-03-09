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
// If no marker is found, heuristically detects the version:
//   - If v1-specific syntax is detected (bare true/false/null, r"..." raw strings), uses v1
//   - Otherwise tries v2 first, falls back to v1 on parse error
func parseSliceAutoDetect(data []byte, opts ParseOptions) (*document.Document, error) {
	// Check for version marker in the input — definitive, no fallback
	if version := detectVersionMarker(data); version != 0 {
		s := tokenizer.NewSlice(data)
		s.RelaxedNonCompliant = opts.RelaxedNonCompliant
		s.ParseComments = opts.Flags.Has(parser.ParseComments)
		s.Version = version
		return parseScanner(s, opts)
	}

	// Heuristic: detect v1-specific syntax
	if hasV1Syntax(data) {
		s := tokenizer.NewSlice(data)
		s.RelaxedNonCompliant = opts.RelaxedNonCompliant
		s.ParseComments = opts.Flags.Has(parser.ParseComments)
		s.Version = 1
		return parseScanner(s, opts)
	}

	// Try v2 first
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	s := tokenizer.NewSlice(dataCopy)
	s.RelaxedNonCompliant = opts.RelaxedNonCompliant
	s.ParseComments = opts.Flags.Has(parser.ParseComments)
	s.Version = 2
	doc, err := parseScanner(s, opts)
	if err == nil {
		return doc, nil
	}

	// V2 failed, fall back to v1
	s = tokenizer.NewSlice(data)
	s.RelaxedNonCompliant = opts.RelaxedNonCompliant
	s.ParseComments = opts.Flags.Has(parser.ParseComments)
	s.Version = 1
	return parseScanner(s, opts)
}

// hasV1Syntax returns true if the data contains syntax that is specific to KDL v1
// and would be interpreted differently (or be invalid) in v2.
// Detects: bare true/false/null keywords, r"..." raw strings
func hasV1Syntax(data []byte) bool {
	return containsBareKeyword(data, []byte("true")) ||
		containsBareKeyword(data, []byte("false")) ||
		containsBareKeyword(data, []byte("null")) ||
		containsV1RawString(data)
}

// containsBareKeyword returns true if data contains keyword as a standalone bare token
// (preceded by whitespace/= and followed by whitespace/newline/;/}/EOF)
func containsBareKeyword(data []byte, keyword []byte) bool {
	for i := 0; i < len(data); {
		idx := bytes.Index(data[i:], keyword)
		if idx == -1 {
			return false
		}
		pos := i + idx
		end := pos + len(keyword)

		// Check preceding char: must be whitespace or = (indicating it's a value, not part of an identifier)
		if pos > 0 {
			prev := data[pos-1]
			if prev != ' ' && prev != '\t' && prev != '=' {
				i = end
				continue
			}
		}

		// Check following char: must be whitespace, newline, ;, }, or EOF
		if end < len(data) {
			next := data[end]
			if next != ' ' && next != '\t' && next != '\n' && next != '\r' &&
				next != ';' && next != '}' && next != '{' {
				i = end
				continue
			}
		}

		return true
	}
	return false
}

// containsV1RawString returns true if data contains v1-style raw strings (r"..." or r#"..."#)
func containsV1RawString(data []byte) bool {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 'r' && (data[i+1] == '"' || data[i+1] == '#') {
			// Check it's not part of a larger identifier
			if i > 0 {
				prev := data[i-1]
				if prev != ' ' && prev != '\t' && prev != '\n' && prev != '\r' &&
					prev != '=' && prev != '(' && prev != '{' {
					continue
				}
			}
			return true
		}
	}
	return false
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
