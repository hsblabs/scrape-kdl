package dom

import (
	"fmt"
	"io"
	"strings"

	xhtml "golang.org/x/net/html"
)

// ParseHTML builds a WHATWG HTML document tree. HTTP response bytes are
// decoded to UTF-8 by the executor before this boundary.
func ParseHTML(r io.Reader) (*Node, error) {
	document, err := xhtml.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	converted := convertHTMLNode(document)
	if converted == nil || converted.Type != DocumentNode {
		return nil, fmt.Errorf("parse HTML: parser did not produce a document")
	}
	return converted, nil
}

func convertHTMLNode(source *xhtml.Node) *Node {
	var node *Node
	switch source.Type {
	case xhtml.DocumentNode:
		node = NewDocument()
	case xhtml.ElementNode:
		node = &Node{Type: ElementNode, Tag: source.Data, Attrs: make(map[string]string, len(source.Attr))}
		for _, attribute := range source.Attr {
			name := attribute.Key
			if attribute.Namespace == "" {
				name = strings.ToLower(name)
			}
			node.Attrs[name] = attribute.Val
		}
	case xhtml.TextNode:
		node = &Node{Type: TextNode, Data: source.Data}
	case xhtml.CommentNode:
		node = &Node{Type: CommentNode, Data: source.Data}
	default:
		return nil
	}
	for child := source.FirstChild; child != nil; child = child.NextSibling {
		if converted := convertHTMLNode(child); converted != nil {
			node.appendChild(converted)
		}
	}
	return node
}

var rawTextElements = map[string]bool{
	"iframe": true, "noembed": true, "noframes": true, "script": true,
	"style": true, "xmp": true,
}
