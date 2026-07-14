package dom

import "testing"

func TestNodeNilAndNonElementBoundaries(t *testing.T) {
	var node *Node
	if value, ok := node.Attr("id"); ok || value != "" {
		t.Fatalf("nil Attr = %q, %v", value, ok)
	}
	if node.TextContent() != "" || node.InnerHTML() != "" || node.ElementChildren() != nil || node.PreviousElementSibling() != nil || !node.IsEmpty() {
		t.Fatal("nil node helpers returned non-empty values")
	}

	text := &Node{Type: TextNode, Data: "value"}
	if value, ok := text.Attr("id"); ok || value != "" {
		t.Fatalf("text Attr = %q, %v", value, ok)
	}
	if text.TextContent() != "value" {
		t.Fatalf("text content = %q", text.TextContent())
	}
}

func TestNodeElementAndSiblingBoundaries(t *testing.T) {
	parent := &Node{Type: ElementNode, Tag: "div"}
	first := &Node{Type: ElementNode, Tag: "span"}
	second := &Node{Type: ElementNode, Tag: "em"}
	parent.appendChild(&Node{Type: TextNode, Data: "before"})
	parent.appendChild(first)
	first.appendChild(&Node{Type: TextNode, Data: "inside"})
	parent.appendChild(&Node{Type: TextNode, Data: "between"})
	parent.appendChild(second)

	children := parent.ElementChildren()
	if len(children) != 2 || children[0] != first || children[1] != second {
		t.Fatalf("element children = %#v", children)
	}
	if first.PreviousElementSibling() != nil || second.PreviousElementSibling() != first {
		t.Fatalf("previous siblings: first=%#v second=%#v", first.PreviousElementSibling(), second.PreviousElementSibling())
	}
	if (&Node{Type: ElementNode}).PreviousElementSibling() != nil {
		t.Fatal("detached node has a previous sibling")
	}
	if parent.TextContent() != "beforeinsidebetween" {
		t.Fatalf("parent text = %q", parent.TextContent())
	}

	empty := &Node{Type: ElementNode, Tag: "div"}
	if !empty.IsEmpty() {
		t.Fatal("childless element is not empty")
	}
	empty.appendChild(&Node{Type: TextNode})
	if !empty.IsEmpty() {
		t.Fatal("zero-length text made element non-empty")
	}
	empty.appendChild(&Node{Type: TextNode, Data: " "})
	if empty.IsEmpty() {
		t.Fatal("whitespace text was treated as empty")
	}
	withElement := &Node{Type: ElementNode, Tag: "div"}
	withElement.appendChild(&Node{Type: ElementNode, Tag: "span"})
	if withElement.IsEmpty() {
		t.Fatal("element child was treated as empty")
	}
}
