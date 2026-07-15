package kdl

import "testing"

func TestParseRawMultilineString(t *testing.T) {
	source := []byte("extractor \"x\" version=\"2026-07-15\" language-version=\"2026-07-15\" {\n  source \"html\" {\n    fetch mode=\"browser\" url=\"https://example.invalid/\"\n    workflow { evaluate-js #\"\"\"\n      () => document.title\n      \"\"\"# }\n  }\n}\n")
	doc, diags := Parse("test.kdl", source)
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
	if len(doc.Nodes) != 1 || len(doc.Nodes[0].Children) != 1 {
		t.Fatalf("unexpected document: %#v", doc)
	}
	workflow := doc.Nodes[0].Children[0].Children[1]
	js := workflow.Children[0]
	if got, ok := stringArgForTest(js, 0); !ok || got == "" {
		t.Fatalf("expected JavaScript string, got %#v", js.Arguments)
	}
}

func stringArgForTest(n *Node, idx int) (string, bool) {
	if idx >= len(n.Arguments) || n.Arguments[idx].Kind != ValueString {
		return "", false
	}
	s, ok := n.Arguments[idx].Value.(string)
	return s, ok
}

func TestDuplicatePropertiesArePreserved(t *testing.T) {
	doc, diags := Parse("test.kdl", []byte("extractor \"x\" version=\"2026-07-15\" version=\"2026-07-16\" language-version=\"2026-07-15\"\n"))
	if diags.HasErrors() {
		t.Fatalf("syntax parser must preserve duplicate properties for semantic validation: %#v", diags)
	}
	if got := len(doc.Nodes[0].PropertiesNamed("version")); got != 2 {
		t.Fatalf("expected two properties, got %d", got)
	}
}

func TestSlashdashSuppressesNodesEntriesAndChildren(t *testing.T) {
	source := []byte(`/-
ignored "node"
kept "first" /- "dropped-argument" "second" /- old=1 new=2 /- { ignored-child } { real-child "ok" }
`)
	doc, diags := Parse("test.kdl", source)
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected one retained node, got %d", len(doc.Nodes))
	}
	node := doc.Nodes[0]
	if node.Name != "kept" {
		t.Fatalf("node name = %q", node.Name)
	}
	if len(node.Arguments) != 2 {
		t.Fatalf("arguments = %#v", node.Arguments)
	}
	if got, _ := stringArgForTest(node, 0); got != "first" {
		t.Fatalf("first argument = %q", got)
	}
	if got, _ := stringArgForTest(node, 1); got != "second" {
		t.Fatalf("second argument = %q", got)
	}
	if len(node.Properties) != 1 || node.Properties[0].Name != "new" {
		t.Fatalf("properties = %#v", node.Properties)
	}
	if len(node.Children) != 1 || node.Children[0].Name != "real-child" {
		t.Fatalf("children = %#v", node.Children)
	}
}

func TestSlashdashCanSuppressTypedComponents(t *testing.T) {
	source := []byte(`/- (ignored) node "x"
kept /- (ignored) "drop" "keep"
`)
	doc, diags := Parse("test.kdl", source)
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
	if len(doc.Nodes) != 1 || len(doc.Nodes[0].Arguments) != 1 {
		t.Fatalf("document = %#v", doc)
	}
	if got, _ := stringArgForTest(doc.Nodes[0], 0); got != "keep" {
		t.Fatalf("argument = %q", got)
	}
}

func TestParseKDLIntegerBases(t *testing.T) {
	doc, diags := Parse("test.kdl", []byte("numbers 0x10 0o10 0b10 -0x2\n"))
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
	want := []int64{16, 8, 2, -2}
	if len(doc.Nodes) != 1 || len(doc.Nodes[0].Arguments) != len(want) {
		t.Fatalf("document = %#v", doc)
	}
	for i, expected := range want {
		if got, ok := doc.Nodes[0].Arguments[i].Value.(int64); !ok || got != expected {
			t.Fatalf("argument[%d] = %#v, want %d", i, doc.Nodes[0].Arguments[i].Value, expected)
		}
	}
}
