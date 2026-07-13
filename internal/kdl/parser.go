package kdl

import (
	"fmt"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/source"
)

type parser struct {
	lex   *lexer
	cur   token
	peek  token
	diags diagnostic.List
}

func Parse(path string, data []byte) (*Document, diagnostic.List) {
	lex := newLexer(path, string(data))
	p := &parser{lex: lex}
	p.cur = lex.next()
	p.peek = lex.next()
	doc := p.parseDocument()
	p.diags = append(p.diags, lex.diags...)
	return doc, p.diags.Sorted()
}

func (p *parser) advance() {
	p.cur = p.peek
	p.peek = p.lex.next()
}

func (p *parser) parseDocument() *Document {
	doc := &Document{Path: p.lex.path}
	start := p.cur.span.Start
	for p.cur.kind != tokenEOF {
		if p.cur.kind == tokenNewline || p.cur.kind == tokenSemicolon {
			p.advance()
			continue
		}
		if p.cur.kind == tokenRBrace {
			p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "unexpected closing brace", p.cur.span, "")
			p.advance()
			continue
		}
		if p.cur.kind == tokenSlashdash {
			p.skipSlashdashedNode()
			continue
		}
		n := p.parseNode()
		if n != nil {
			doc.Nodes = append(doc.Nodes, n)
		}
	}
	end := p.cur.span.End
	doc.Span = source.Span{File: p.lex.path, Start: start, End: end}
	return doc
}

func (p *parser) parseNode() *Node {
	if p.cur.kind == tokenLParen {
		span := p.cur.span
		p.skipTypeAnnotation()
		p.diags.Add("E_TYPE_ANNOTATION_UNSUPPORTED", diagnostic.SeverityError, "KDL type annotations are not supported", span, "")
	}
	if p.cur.kind != tokenIdentifier && p.cur.kind != tokenString {
		p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "expected node name", p.cur.span, "")
		p.recoverNode()
		return nil
	}
	start := p.cur.span
	name := p.cur.text
	if p.cur.kind == tokenString {
		name, _ = p.cur.value.(string)
	}
	p.advance()
	n := &Node{Name: name}

	for {
		switch p.cur.kind {
		case tokenEOF, tokenNewline, tokenSemicolon, tokenRBrace:
			n.Span = source.Merge(start, previousEnd(start, n))
			if p.cur.kind == tokenNewline || p.cur.kind == tokenSemicolon {
				p.advance()
			}
			return n
		case tokenLBrace:
			open := p.cur.span
			p.advance()
			n.Children = p.parseChildren()
			if p.cur.kind == tokenRBrace {
				n.Span = source.Merge(start, p.cur.span)
				p.advance()
			} else {
				p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "unterminated child block", open, "")
				n.Span = source.Merge(start, open)
			}
			if p.cur.kind == tokenNewline || p.cur.kind == tokenSemicolon {
				p.advance()
			}
			return n
		case tokenLParen:
			span := p.cur.span
			p.skipTypeAnnotation()
			p.diags.Add("E_TYPE_ANNOTATION_UNSUPPORTED", diagnostic.SeverityError, "KDL type annotations are not supported", span, "")
		case tokenSlashdash:
			p.skipSlashdashedComponent()
		default:
			if p.cur.kind == tokenIdentifier && p.peek.kind == tokenEquals {
				nameTok := p.cur
				p.advance()
				p.advance()
				v, ok := p.parseValue()
				if ok {
					n.Properties = append(n.Properties, Property{Name: nameTok.text, Value: v, Span: source.Merge(nameTok.span, v.Span)})
				}
				continue
			}
			v, ok := p.parseValue()
			if ok {
				n.Arguments = append(n.Arguments, v)
				continue
			}
			p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, fmt.Sprintf("unexpected token %q in node", p.cur.text), p.cur.span, "")
			p.advance()
		}
	}
}

func (p *parser) parseChildren() []*Node {
	var nodes []*Node
	for p.cur.kind != tokenEOF && p.cur.kind != tokenRBrace {
		if p.cur.kind == tokenNewline || p.cur.kind == tokenSemicolon {
			p.advance()
			continue
		}
		if p.cur.kind == tokenSlashdash {
			p.skipSlashdashedNode()
			continue
		}
		n := p.parseNode()
		if n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func (p *parser) skipSlashdashedNode() {
	marker := p.cur.span
	p.advance()
	p.skipSlashdashLineSpace()
	if p.cur.kind == tokenSlashdash {
		p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "a slashdash cannot directly suppress another slashdash", p.cur.span, "")
		p.advance()
		return
	}
	if p.cur.kind == tokenLParen {
		p.skipTypeAnnotation()
		p.skipSlashdashLineSpace()
	}
	if p.cur.kind != tokenIdentifier && p.cur.kind != tokenString {
		p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "slashdash before a node must be followed by a node", marker, "")
		return
	}
	p.parseNode()
}

func (p *parser) skipSlashdashedComponent() {
	marker := p.cur.span
	p.advance()
	p.skipSlashdashLineSpace()
	if p.cur.kind == tokenSlashdash {
		p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "a slashdash cannot directly suppress another slashdash", p.cur.span, "")
		p.advance()
		return
	}
	if p.cur.kind == tokenLParen {
		p.skipTypeAnnotation()
		p.skipSlashdashLineSpace()
	}
	switch {
	case p.cur.kind == tokenLBrace:
		p.skipChildBlock()
	case (p.cur.kind == tokenIdentifier || p.cur.kind == tokenString) && p.peek.kind == tokenEquals:
		p.advance() // property name
		p.advance() // equals
		if p.cur.kind == tokenLParen {
			p.skipTypeAnnotation()
		}
		if _, ok := p.parseValue(); !ok {
			p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "slashdashed property is missing a value", marker, "")
		}
	default:
		if _, ok := p.parseValue(); !ok {
			p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "slashdash inside a node must suppress an argument, property, or children block", marker, "")
		}
	}
}

func (p *parser) skipSlashdashLineSpace() {
	for p.cur.kind == tokenNewline {
		p.advance()
	}
}

func (p *parser) skipChildBlock() {
	open := p.cur.span
	p.advance()
	_ = p.parseChildren()
	if p.cur.kind != tokenRBrace {
		p.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "unterminated slashdashed child block", open, "")
		return
	}
	p.advance()
}

func (p *parser) parseValue() (Value, bool) {
	t := p.cur
	var kind ValueKind
	switch t.kind {
	case tokenString:
		kind = ValueString
	case tokenInt:
		kind = ValueInt
	case tokenFloat:
		kind = ValueFloat
	case tokenBool:
		kind = ValueBool
	case tokenNull:
		kind = ValueNull
	default:
		return Value{}, false
	}
	p.advance()
	return Value{Kind: kind, Raw: t.text, Value: t.value, Span: t.span}, true
}

func (p *parser) skipTypeAnnotation() {
	depth := 0
	for p.cur.kind != tokenEOF {
		if p.cur.kind == tokenLParen {
			depth++
		}
		if p.cur.kind == tokenRParen {
			depth--
			p.advance()
			if depth <= 0 {
				return
			}
			continue
		}
		p.advance()
	}
}

func (p *parser) recoverNode() {
	for p.cur.kind != tokenEOF && p.cur.kind != tokenNewline && p.cur.kind != tokenSemicolon && p.cur.kind != tokenRBrace {
		p.advance()
	}
	if p.cur.kind == tokenNewline || p.cur.kind == tokenSemicolon {
		p.advance()
	}
}

func previousEnd(start source.Span, n *Node) source.Span {
	end := start
	if len(n.Properties) > 0 {
		end = n.Properties[len(n.Properties)-1].Span
	}
	if len(n.Arguments) > 0 {
		end = n.Arguments[len(n.Arguments)-1].Span
	}
	return end
}
