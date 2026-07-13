package dom

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// ParseHTML parses common HTML using the standard library's permissive XML
// tokenizer plus a small HTML normalization layer. It intentionally keeps the
// runtime dependency-free. The normalizer handles raw-text/RCDATA elements,
// common optional end tags, void elements, and truncated documents. It is not
// a complete WHATWG tree builder and does not implement foster parenting around
// malformed tables or the full foreign-content algorithm.
func ParseHTML(r io.Reader) (*Node, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read HTML: %w", err)
	}
	prepared := protectRawText(string(raw))
	decoder := xml.NewDecoder(strings.NewReader(prepared))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	document := NewDocument()
	stack := []*Node{document}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Browsers implicitly close open elements at end of input. The XML
			// tokenizer reports this as an unexpected EOF after it has already
			// emitted all useful tokens, so preserve the recovered tree.
			if strings.Contains(err.Error(), "unexpected EOF") {
				break
			}
			return nil, fmt.Errorf("parse HTML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			tag := strings.ToLower(value.Name.Local)
			stack = closeOptionalElements(stack, tag)
			node := &Node{Type: ElementNode, Tag: tag, Attrs: make(map[string]string, len(value.Attr))}
			for _, attribute := range value.Attr {
				name := strings.ToLower(attribute.Name.Local)
				node.Attrs[name] = attribute.Value
			}
			stack[len(stack)-1].appendChild(node)
			if _, isVoid := voidElements[tag]; !isVoid {
				stack = append(stack, node)
			}
		case xml.EndElement:
			tag := strings.ToLower(value.Name.Local)
			for i := len(stack) - 1; i > 0; i-- {
				if stack[i].Tag == tag {
					stack = stack[:i]
					break
				}
			}
		case xml.CharData:
			if len(value) == 0 {
				continue
			}
			stack[len(stack)-1].appendChild(&Node{Type: TextNode, Data: string(value)})
		}
	}
	return document, nil
}

func closeOptionalElements(stack []*Node, incoming string) []*Node {
	if len(stack) <= 1 {
		return stack
	}
	for len(stack) > 1 {
		open := stack[len(stack)-1].Tag
		if !shouldImplicitlyClose(open, incoming) {
			break
		}
		stack = stack[:len(stack)-1]
	}
	return stack
}

func shouldImplicitlyClose(open, incoming string) bool {
	switch open {
	case "li":
		return incoming == "li"
	case "dt", "dd":
		return incoming == "dt" || incoming == "dd"
	case "p":
		return isParagraphClosingStart(incoming)
	case "rt", "rp":
		return incoming == "rt" || incoming == "rp"
	case "optgroup":
		return incoming == "optgroup"
	case "option":
		return incoming == "option" || incoming == "optgroup"
	case "thead":
		return incoming == "tbody" || incoming == "tfoot"
	case "tbody", "tfoot":
		return incoming == "tbody" || incoming == "tfoot"
	case "tr":
		return incoming == "tr"
	case "td", "th":
		return incoming == "td" || incoming == "th" || incoming == "tr"
	}
	return false
}

func isParagraphClosingStart(tag string) bool {
	switch tag {
	case "address", "article", "aside", "blockquote", "details", "dialog", "div", "dl", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hgroup", "hr", "main", "menu", "nav", "ol", "p", "pre", "search", "section", "table", "ul":
		return true
	default:
		return false
	}
}

var rawTextElements = map[string]bool{
	"iframe":   true,
	"noembed":  true,
	"noframes": true,
	"script":   true,
	"style":    true,
	"xmp":      true,
}

var rcdataElements = map[string]bool{
	"textarea": true,
	"title":    true,
}

// protectRawText escapes markup-significant bytes inside HTML raw-text and
// RCDATA elements before the XML tokenizer sees them. XML entity decoding then
// restores the original text in the resulting DOM.
func protectRawText(source string) string {
	lower := strings.ToLower(source)
	var out strings.Builder
	out.Grow(len(source))

	for cursor := 0; cursor < len(source); {
		if source[cursor] != '<' {
			out.WriteByte(source[cursor])
			cursor++
			continue
		}
		end := findTagEnd(source, cursor)
		if end < 0 {
			out.WriteString(source[cursor:])
			break
		}
		tag, closing, selfClosing := parseTagName(source[cursor : end+1])
		out.WriteString(source[cursor : end+1])
		cursor = end + 1
		if closing || selfClosing || (!rawTextElements[tag] && !rcdataElements[tag]) {
			continue
		}

		closeStart := findClosingTag(lower, cursor, tag)
		if closeStart < 0 {
			closeStart = len(source)
		}
		content := source[cursor:closeStart]
		if rawTextElements[tag] {
			content = strings.ReplaceAll(content, "&", "&amp;")
		}
		content = strings.ReplaceAll(content, "<", "&lt;")
		out.WriteString(content)
		cursor = closeStart
	}
	return out.String()
}

func findTagEnd(source string, start int) int {
	var quote byte
	for i := start + 1; i < len(source); i++ {
		c := source[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '>':
			return i
		}
	}
	return -1
}

func parseTagName(tagSource string) (name string, closing, selfClosing bool) {
	if len(tagSource) < 3 || tagSource[0] != '<' {
		return "", false, false
	}
	i := 1
	for i < len(tagSource) && unicode.IsSpace(rune(tagSource[i])) {
		i++
	}
	if i < len(tagSource) && tagSource[i] == '/' {
		closing = true
		i++
	}
	if i >= len(tagSource) || tagSource[i] == '!' || tagSource[i] == '?' {
		return "", closing, false
	}
	start := i
	for i < len(tagSource) {
		c := tagSource[i]
		if !(c == '-' || c == ':' || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			break
		}
		i++
	}
	name = strings.ToLower(tagSource[start:i])
	trimmed := strings.TrimSpace(tagSource)
	selfClosing = strings.HasSuffix(trimmed, "/>")
	return name, closing, selfClosing
}

func findClosingTag(lower string, start int, tag string) int {
	needle := "</" + tag
	for start < len(lower) {
		relative := strings.Index(lower[start:], needle)
		if relative < 0 {
			return -1
		}
		candidate := start + relative
		after := candidate + len(needle)
		if after >= len(lower) || lower[after] == '>' || lower[after] == '/' || unicode.IsSpace(rune(lower[after])) {
			return candidate
		}
		start = candidate + 2
	}
	return -1
}
