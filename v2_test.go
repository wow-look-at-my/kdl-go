package kdl

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/sblinch/kdl-go/document"
	"github.com/sblinch/kdl-go/internal/generator"
	"github.com/sblinch/kdl-go/internal/parser"
)

func TestV2Keywords(t *testing.T) {
	input := `node #true #false #null`

	doc, err := ParseSliceWithOptions([]byte(input), ParseOptions{Version: 2})
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(n.Arguments))
	}

	// #true -> bool true
	if v, ok := n.Arguments[0].Value.(bool); !ok || !v {
		t.Errorf("expected #true -> bool true, got %T %v", n.Arguments[0].Value, n.Arguments[0].Value)
	}

	// #false -> bool false
	if v, ok := n.Arguments[1].Value.(bool); !ok || v {
		t.Errorf("expected #false -> bool false, got %T %v", n.Arguments[1].Value, n.Arguments[1].Value)
	}

	// #null -> nil
	if n.Arguments[2].Value != nil {
		t.Errorf("expected #null -> nil, got %T %v", n.Arguments[2].Value, n.Arguments[2].Value)
	}
}

func TestV2BareKeywordsAreIdentifiers(t *testing.T) {
	// In v2, bare true/false/null are plain identifiers, not keywords
	input := `node true false null`

	doc, err := ParseSliceWithOptions([]byte(input), ParseOptions{Version: 2})
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(n.Arguments))
	}

	for i, expected := range []string{"true", "false", "null"} {
		if v, ok := n.Arguments[i].Value.(string); !ok || v != expected {
			t.Errorf("arg %d: expected string %q, got %T %v", i, expected, n.Arguments[i].Value, n.Arguments[i].Value)
		}
	}
}

func TestV2SpecialFloats(t *testing.T) {
	input := `node #inf #-inf #nan`

	doc, err := ParseSliceWithOptions([]byte(input), ParseOptions{Version: 2})
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(n.Arguments))
	}

	// #inf -> +Inf
	if v, ok := n.Arguments[0].Value.(float64); !ok || !math.IsInf(v, 1) {
		t.Errorf("expected #inf -> +Inf, got %T %v", n.Arguments[0].Value, n.Arguments[0].Value)
	}

	// #-inf -> -Inf
	if v, ok := n.Arguments[1].Value.(float64); !ok || !math.IsInf(v, -1) {
		t.Errorf("expected #-inf -> -Inf, got %T %v", n.Arguments[1].Value, n.Arguments[1].Value)
	}

	// #nan -> NaN
	if v, ok := n.Arguments[2].Value.(float64); !ok || !math.IsNaN(v) {
		t.Errorf("expected #nan -> NaN, got %T %v", n.Arguments[2].Value, n.Arguments[2].Value)
	}
}

func TestV2RawStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", `node #"hello world"#`, "hello world"},
		{"with quotes", `node #"hello "world""#`, `hello "world"`},
		{"with backslash", `node #"hello\nworld"#`, `hello\nworld`},
		{"multi-hash", `node ##"hello "#world"##`, `hello "#world`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := ParseSliceWithOptions([]byte(tt.input), ParseOptions{Version: 2})
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			n := doc.Nodes[0]
			if len(n.Arguments) != 1 {
				t.Fatalf("expected 1 argument, got %d", len(n.Arguments))
			}

			if v, ok := n.Arguments[0].Value.(string); !ok || v != tt.expected {
				t.Errorf("expected %q, got %T %v", tt.expected, n.Arguments[0].Value, n.Arguments[0].Value)
			}
		})
	}
}

func TestV2VersionMarker(t *testing.T) {
	// With version marker, should parse as v2
	input := `/- kdl-version 2
node #true #false #null`

	doc, err := ParseSlice([]byte(input))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(n.Arguments))
	}

	// With version marker, #true should be bool true
	if v, ok := n.Arguments[0].Value.(bool); !ok || !v {
		t.Errorf("expected #true -> bool true, got %T %v", n.Arguments[0].Value, n.Arguments[0].Value)
	}
}

func TestV1VersionMarker(t *testing.T) {
	// With v1 version marker, should parse as v1
	input := `/- kdl-version 1
node true false null`

	doc, err := ParseSlice([]byte(input))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(n.Arguments))
	}

	// v1: bare true is boolean
	if v, ok := n.Arguments[0].Value.(bool); !ok || !v {
		t.Errorf("expected true -> bool true, got %T %v", n.Arguments[0].Value, n.Arguments[0].Value)
	}
}

func TestAutoDetectV1Syntax(t *testing.T) {
	// Documents with bare true/false/null should be detected as v1
	input := `node true false null`

	doc, err := ParseSlice([]byte(input))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(n.Arguments))
	}

	// Auto-detected as v1, so bare true is boolean
	if v, ok := n.Arguments[0].Value.(bool); !ok || !v {
		t.Errorf("expected true -> bool true (auto-detected v1), got %T %v", n.Arguments[0].Value, n.Arguments[0].Value)
	}
}

func TestAutoDetectV2Syntax(t *testing.T) {
	// Documents with #true/#false/#null should be parsed as v2
	input := `node #true #false #null`

	doc, err := ParseSlice([]byte(input))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(n.Arguments))
	}

	// Should auto-detect as v2 since there's no v1-specific syntax
	if v, ok := n.Arguments[0].Value.(bool); !ok || !v {
		t.Errorf("expected #true -> bool true, got %T %v", n.Arguments[0].Value, n.Arguments[0].Value)
	}
}

func TestV2GenerateOutput(t *testing.T) {
	// Create a document with v2 values and generate v2 output
	doc := document.New()
	doc.Version = document.VersionV2

	n := document.NewNode()
	n.SetName("node")
	n.AddArgument(true, "")
	n.AddArgument(false, "")
	n.AddArgument(nil, "")
	n.AddArgument(math.Inf(1), "")
	n.AddArgument(math.Inf(-1), "")
	n.AddArgument(math.NaN(), "")
	doc.Nodes = append(doc.Nodes, n)

	var buf bytes.Buffer
	opts := generator.DefaultOptions
	opts.Version = 2
	g := generator.NewOptions(&buf, opts)
	if err := g.Generate(doc); err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	output := buf.String()

	// Should start with version marker
	if !strings.HasPrefix(output, "/- kdl-version 2\n") {
		t.Errorf("v2 output should start with version marker, got:\n%s", output)
	}

	// Should contain v2 keywords
	if !strings.Contains(output, "#true") {
		t.Errorf("expected #true in output, got:\n%s", output)
	}
	if !strings.Contains(output, "#false") {
		t.Errorf("expected #false in output, got:\n%s", output)
	}
	if !strings.Contains(output, "#null") {
		t.Errorf("expected #null in output, got:\n%s", output)
	}
	if !strings.Contains(output, "#inf") {
		t.Errorf("expected #inf in output, got:\n%s", output)
	}
	if !strings.Contains(output, "#-inf") {
		t.Errorf("expected #-inf in output, got:\n%s", output)
	}
	if !strings.Contains(output, "#nan") {
		t.Errorf("expected #nan in output, got:\n%s", output)
	}
}

func TestV2RoundTrip(t *testing.T) {
	input := `/- kdl-version 2
node #true #false #null
floats #inf #-inf #nan
raw #"hello world"#
nested {
	child 42 key="value"
}
`

	// Parse
	doc, err := ParseSlice([]byte(input))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Generate
	var buf bytes.Buffer
	opts := generator.DefaultOptions
	opts.Version = 2
	g := generator.NewOptions(&buf, opts)
	if err := g.Generate(doc); err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	output := buf.String()

	// Re-parse the generated output
	doc2, err := ParseSlice([]byte(output))
	if err != nil {
		t.Fatalf("failed to re-parse: %v\nOutput was:\n%s", err, output)
	}

	// Compare documents structurally
	if len(doc.Nodes) != len(doc2.Nodes) {
		t.Fatalf("node count mismatch: %d vs %d", len(doc.Nodes), len(doc2.Nodes))
	}

	// Check first node (node #true #false #null)
	n1 := doc.Nodes[0]
	n2 := doc2.Nodes[0]
	if len(n1.Arguments) != len(n2.Arguments) {
		t.Fatalf("arg count mismatch for node 0: %d vs %d", len(n1.Arguments), len(n2.Arguments))
	}
	for i := range n1.Arguments {
		v1 := n1.Arguments[i].Value
		v2 := n2.Arguments[i].Value
		// Special handling for NaN
		if f1, ok := v1.(float64); ok && math.IsNaN(f1) {
			if f2, ok := v2.(float64); ok && math.IsNaN(f2) {
				continue
			}
		}
		if v1 != v2 {
			t.Errorf("node 0 arg %d: %v != %v", i, v1, v2)
		}
	}
}

func TestV2MarshalUnmarshal(t *testing.T) {
	type Config struct {
		Name    string `kdl:"name"`
		Enabled bool   `kdl:"enabled"`
		Count   int    `kdl:"count"`
	}

	original := &Config{
		Name:    "test",
		Enabled: true,
		Count:   42,
	}

	// Marshal
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	result := &Config{}
	if err := Unmarshal(data, result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Name != original.Name || result.Enabled != original.Enabled || result.Count != original.Count {
		t.Errorf("round-trip mismatch:\n  original: %+v\n  result:   %+v", original, result)
	}
}

func TestV2MarshalWithVersion(t *testing.T) {
	type Config struct {
		Name    string `kdl:"name"`
		Enabled bool   `kdl:"enabled"`
	}

	original := &Config{
		Name:    "test",
		Enabled: true,
	}

	// Marshal with v2
	opts := MarshalOptions{}
	opts.GeneratorOptions.Version = 2
	data, err := MarshalWithOptions(original, opts)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	output := string(data)
	if !strings.HasPrefix(output, "/- kdl-version 2\n") {
		t.Errorf("v2 marshal should start with version marker, got:\n%s", output)
	}
	if !strings.Contains(output, "#true") {
		t.Errorf("v2 marshal should use #true, got:\n%s", output)
	}
}

func TestV2ParseComments(t *testing.T) {
	input := `/- kdl-version 2
// This is a comment
node 42
`

	doc, err := ParseSliceWithOptions([]byte(input), ParseOptions{
		Flags: parser.ParseComments,
	})
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}
}

func TestV2Properties(t *testing.T) {
	input := `node key=#true val=#false nothing=#null`

	doc, err := ParseSliceWithOptions([]byte(input), ParseOptions{Version: 2})
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]

	if v, ok := n.Properties.Get("key"); !ok {
		t.Error("missing property 'key'")
	} else if b, ok := v.Value.(bool); !ok || !b {
		t.Errorf("expected key=#true -> true, got %T %v", v.Value, v.Value)
	}

	if v, ok := n.Properties.Get("val"); !ok {
		t.Error("missing property 'val'")
	} else if b, ok := v.Value.(bool); !ok || b {
		t.Errorf("expected val=#false -> false, got %T %v", v.Value, v.Value)
	}

	if v, ok := n.Properties.Get("nothing"); !ok {
		t.Error("missing property 'nothing'")
	} else if v.Value != nil {
		t.Errorf("expected nothing=#null -> nil, got %T %v", v.Value, v.Value)
	}
}

func TestV1RawStringAutoDetect(t *testing.T) {
	// Documents with r"..." should be auto-detected as v1
	input := `node r"hello\nworld"`

	doc, err := ParseSlice([]byte(input))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	n := doc.Nodes[0]
	if len(n.Arguments) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(n.Arguments))
	}

	// r"..." in v1 means raw string, so backslash-n is literal
	if v, ok := n.Arguments[0].Value.(string); !ok || v != `hello\nworld` {
		t.Errorf("expected raw string %q, got %T %v", `hello\nworld`, n.Arguments[0].Value, n.Arguments[0].Value)
	}
}
