package dom

import (
	"html"
	"sort"
	"strings"
)

type NodeType uint8

const (
	DocumentNode NodeType = iota
	ElementNode
	TextNode
	CommentNode
)

type Node struct {
	Type     NodeType
	Tag      string
	Data     string
	Attrs    map[string]string
	Parent   *Node
	Children []*Node
}

func NewDocument() *Node { return &Node{Type: DocumentNode} }

func (n *Node) appendChild(child *Node) {
	child.Parent = n
	n.Children = append(n.Children, child)
}

func (n *Node) Attr(name string) (string, bool) {
	if n == nil || n.Type != ElementNode {
		return "", false
	}
	value, ok := n.Attrs[strings.ToLower(name)]
	return value, ok
}

func (n *Node) TextContent() string {
	if n == nil {
		return ""
	}
	if n.Type == TextNode {
		return n.Data
	}
	var b strings.Builder
	var walk func(*Node)
	walk = func(node *Node) {
		for _, child := range node.Children {
			if child.Type == TextNode {
				b.WriteString(child.Data)
			} else {
				walk(child)
			}
		}
	}
	walk(n)
	return b.String()
}

func (n *Node) InnerHTML() string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	for _, child := range n.Children {
		serializeNode(&b, child)
	}
	return b.String()
}

var voidElements = map[string]struct{}{
	"area": {}, "base": {}, "br": {}, "col": {}, "embed": {}, "hr": {}, "img": {},
	"input": {}, "link": {}, "meta": {}, "param": {}, "source": {}, "track": {}, "wbr": {},
}

func serializeNode(b *strings.Builder, n *Node) {
	switch n.Type {
	case TextNode:
		if n.Parent != nil && rawTextElements[n.Parent.Tag] {
			b.WriteString(n.Data)
		} else {
			b.WriteString(html.EscapeString(n.Data))
		}
	case CommentNode:
		b.WriteString("<!--")
		b.WriteString(n.Data)
		b.WriteString("-->")
	case ElementNode:
		b.WriteByte('<')
		b.WriteString(n.Tag)
		keys := make([]string, 0, len(n.Attrs))
		for key := range n.Attrs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteByte(' ')
			b.WriteString(key)
			b.WriteString(`="`)
			b.WriteString(html.EscapeString(n.Attrs[key]))
			b.WriteByte('"')
		}
		b.WriteByte('>')
		if _, ok := voidElements[n.Tag]; ok {
			return
		}
		for _, child := range n.Children {
			serializeNode(b, child)
		}
		b.WriteString("</")
		b.WriteString(n.Tag)
		b.WriteByte('>')
	}
}

func (n *Node) ElementChildren() []*Node {
	if n == nil {
		return nil
	}
	children := make([]*Node, 0, len(n.Children))
	for _, child := range n.Children {
		if child.Type == ElementNode {
			children = append(children, child)
		}
	}
	return children
}

func (n *Node) PreviousElementSibling() *Node {
	if n == nil || n.Parent == nil {
		return nil
	}
	var previous *Node
	for _, child := range n.Parent.Children {
		if child == n {
			return previous
		}
		if child.Type == ElementNode {
			previous = child
		}
	}
	return nil
}

func (n *Node) IsEmpty() bool {
	if n == nil {
		return true
	}
	for _, child := range n.Children {
		if child.Type == ElementNode {
			return false
		}
		if child.Type == TextNode && child.Data != "" {
			return false
		}
	}
	return true
}
