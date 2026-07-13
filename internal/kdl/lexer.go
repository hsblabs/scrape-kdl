package kdl

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/source"
)

type lexer struct {
	path   string
	input  string
	offset int
	line   int
	column int
	diags  diagnostic.List
}

func newLexer(path, input string) *lexer {
	return &lexer{path: path, input: input, line: 1, column: 1}
}

func (l *lexer) pos() source.Position {
	return source.Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func (l *lexer) span(start source.Position) source.Span {
	return source.Span{File: l.path, Start: start, End: l.pos()}
}

func (l *lexer) eof() bool { return l.offset >= len(l.input) }

func (l *lexer) peekByte(n int) byte {
	idx := l.offset + n
	if idx < 0 || idx >= len(l.input) {
		return 0
	}
	return l.input[idx]
}

func (l *lexer) advance() rune {
	if l.eof() {
		return 0
	}
	r, size := utf8.DecodeRuneInString(l.input[l.offset:])
	l.offset += size
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return r
}

func (l *lexer) next() token {
	for {
		if l.eof() {
			p := l.pos()
			return token{kind: tokenEOF, span: source.Span{File: l.path, Start: p, End: p}}
		}

		start := l.pos()
		ch := l.peekByte(0)
		switch ch {
		case ' ', '\t', '\r':
			l.advance()
			continue
		case '\n':
			l.advance()
			return token{kind: tokenNewline, text: "\n", span: l.span(start)}
		case ';':
			l.advance()
			return token{kind: tokenSemicolon, text: ";", span: l.span(start)}
		case '{':
			l.advance()
			return token{kind: tokenLBrace, text: "{", span: l.span(start)}
		case '}':
			l.advance()
			return token{kind: tokenRBrace, text: "}", span: l.span(start)}
		case '=':
			l.advance()
			return token{kind: tokenEquals, text: "=", span: l.span(start)}
		case '(':
			l.advance()
			return token{kind: tokenLParen, text: "(", span: l.span(start)}
		case ')':
			l.advance()
			return token{kind: tokenRParen, text: ")", span: l.span(start)}
		case '/':
			if l.peekByte(1) == '/' {
				l.skipLineComment()
				continue
			}
			if l.peekByte(1) == '*' {
				l.skipBlockComment(start)
				continue
			}
			if l.peekByte(1) == '-' {
				l.advance()
				l.advance()
				return token{kind: tokenSlashdash, text: "/-", span: l.span(start)}
			}
		case '"':
			return l.scanQuotedString(start)
		case '#':
			return l.scanHashToken(start)
		}

		if isNumberStart(ch, l.peekByte(1)) {
			return l.scanNumber(start)
		}
		if isIdentifierStart(ch) {
			return l.scanIdentifier(start)
		}

		bad := l.advance()
		span := l.span(start)
		l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, fmt.Sprintf("unexpected character %q", bad), span, "")
		return token{kind: tokenInvalid, text: string(bad), span: span}
	}
}

func (l *lexer) skipLineComment() {
	for !l.eof() && l.peekByte(0) != '\n' {
		l.advance()
	}
}

func (l *lexer) skipBlockComment(start source.Position) {
	l.advance()
	l.advance()
	depth := 1
	for !l.eof() {
		if l.peekByte(0) == '/' && l.peekByte(1) == '*' {
			l.advance()
			l.advance()
			depth++
			continue
		}
		if l.peekByte(0) == '*' && l.peekByte(1) == '/' {
			l.advance()
			l.advance()
			depth--
			if depth == 0 {
				return
			}
			continue
		}
		l.advance()
	}
	l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "unterminated block comment", l.span(start), "")
}

func (l *lexer) scanIdentifier(start source.Position) token {
	begin := l.offset
	for !l.eof() && isIdentifierContinue(l.peekByte(0)) {
		l.advance()
	}
	text := l.input[begin:l.offset]
	return token{kind: tokenIdentifier, text: text, value: text, span: l.span(start)}
}

func (l *lexer) scanQuotedString(start source.Position) token {
	l.advance()
	var b strings.Builder
	for !l.eof() {
		r := l.advance()
		switch r {
		case '"':
			return token{kind: tokenString, text: l.input[start.Offset:l.offset], value: b.String(), span: l.span(start)}
		case '\\':
			if l.eof() {
				break
			}
			esc := l.advance()
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case 'u':
				if l.peekByte(0) == '{' {
					l.advance()
					hexStart := l.offset
					for !l.eof() && l.peekByte(0) != '}' {
						l.advance()
					}
					if l.eof() {
						l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "unterminated Unicode escape", l.span(start), "")
						return token{kind: tokenInvalid, span: l.span(start)}
					}
					hex := l.input[hexStart:l.offset]
					l.advance()
					v, err := strconv.ParseInt(hex, 16, 32)
					if err != nil || !utf8.ValidRune(rune(v)) {
						l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "invalid Unicode escape", l.span(start), "")
						return token{kind: tokenInvalid, span: l.span(start)}
					}
					b.WriteRune(rune(v))
				} else {
					l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "KDL 2 Unicode escapes must use \\u{...}", l.span(start), "")
					return token{kind: tokenInvalid, span: l.span(start)}
				}
			default:
				l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, fmt.Sprintf("invalid string escape \\%c", esc), l.span(start), "")
				return token{kind: tokenInvalid, span: l.span(start)}
			}
		case '\n', '\r':
			l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "newline in quoted string", l.span(start), "")
			return token{kind: tokenInvalid, span: l.span(start)}
		default:
			b.WriteRune(r)
		}
	}
	l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "unterminated quoted string", l.span(start), "")
	return token{kind: tokenInvalid, span: l.span(start)}
}

func (l *lexer) scanHashToken(start source.Position) token {
	if strings.HasPrefix(l.input[l.offset:], "#true") && isBoundary(l.peekByte(5)) {
		for range 5 {
			l.advance()
		}
		return token{kind: tokenBool, text: "#true", value: true, span: l.span(start)}
	}
	if strings.HasPrefix(l.input[l.offset:], "#false") && isBoundary(l.peekByte(6)) {
		for range 6 {
			l.advance()
		}
		return token{kind: tokenBool, text: "#false", value: false, span: l.span(start)}
	}
	if strings.HasPrefix(l.input[l.offset:], "#null") && isBoundary(l.peekByte(5)) {
		for range 5 {
			l.advance()
		}
		return token{kind: tokenNull, text: "#null", value: nil, span: l.span(start)}
	}

	hashes := 0
	for l.peekByte(hashes) == '#' {
		hashes++
	}
	if l.peekByte(hashes) != '"' {
		l.advance()
		l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "invalid hash-prefixed token", l.span(start), "")
		return token{kind: tokenInvalid, span: l.span(start)}
	}
	for range hashes {
		l.advance()
	}
	triple := l.peekByte(0) == '"' && l.peekByte(1) == '"' && l.peekByte(2) == '"'
	quoteCount := 1
	if triple {
		quoteCount = 3
	}
	for range quoteCount {
		l.advance()
	}
	contentStart := l.offset
	for !l.eof() {
		if quoteCount == 3 {
			if l.peekByte(0) == '"' && l.peekByte(1) == '"' && l.peekByte(2) == '"' {
				ok := true
				for i := 0; i < hashes; i++ {
					if l.peekByte(3+i) != '#' {
						ok = false
						break
					}
				}
				if ok {
					content := l.input[contentStart:l.offset]
					for range 3 + hashes {
						l.advance()
					}
					content = normalizeRawMultiline(content)
					return token{kind: tokenString, text: l.input[start.Offset:l.offset], value: content, span: l.span(start)}
				}
			}
		} else if l.peekByte(0) == '"' {
			ok := true
			for i := 0; i < hashes; i++ {
				if l.peekByte(1+i) != '#' {
					ok = false
					break
				}
			}
			if ok {
				content := l.input[contentStart:l.offset]
				for range 1 + hashes {
					l.advance()
				}
				return token{kind: tokenString, text: l.input[start.Offset:l.offset], value: content, span: l.span(start)}
			}
		}
		l.advance()
	}
	l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "unterminated raw string", l.span(start), "")
	return token{kind: tokenInvalid, span: l.span(start)}
}

func normalizeRawMultiline(s string) string {
	if strings.HasPrefix(s, "\r\n") {
		s = s[2:]
	} else if strings.HasPrefix(s, "\n") {
		s = s[1:]
	}
	if strings.HasSuffix(s, "\r\n") {
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	return s
}

func (l *lexer) scanNumber(start source.Position) token {
	begin := l.offset
	if l.peekByte(0) == '+' || l.peekByte(0) == '-' {
		l.advance()
	}

	// KDL 2 integer literals may use hexadecimal, octal, or binary prefixes.
	if l.peekByte(0) == '0' {
		prefix := l.peekByte(1)
		base := 0
		validDigit := func(byte) bool { return false }
		switch prefix {
		case 'x', 'X':
			base = 16
			validDigit = isHexDigit
		case 'o', 'O':
			base = 8
			validDigit = func(ch byte) bool { return ch >= '0' && ch <= '7' }
		case 'b', 'B':
			base = 2
			validDigit = func(ch byte) bool { return ch == '0' || ch == '1' }
		}
		if base != 0 {
			l.advance()
			l.advance()
			digits := 0
			for validDigit(l.peekByte(0)) || l.peekByte(0) == '_' {
				if l.peekByte(0) != '_' {
					digits++
				}
				l.advance()
			}
			raw := l.input[begin:l.offset]
			if digits == 0 {
				l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "integer base prefix must be followed by a digit", l.span(start), "")
				return token{kind: tokenInvalid, text: raw, span: l.span(start)}
			}
			clean := strings.ReplaceAll(raw, "_", "")
			v, err := strconv.ParseInt(clean, 0, 64)
			if err != nil {
				l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "invalid integer literal", l.span(start), "")
				return token{kind: tokenInvalid, text: raw, span: l.span(start)}
			}
			return token{kind: tokenInt, text: raw, value: v, span: l.span(start)}
		}
	}

	for isDigit(l.peekByte(0)) || l.peekByte(0) == '_' {
		l.advance()
	}
	kind := tokenInt
	if l.peekByte(0) == '.' {
		kind = tokenFloat
		l.advance()
		for isDigit(l.peekByte(0)) || l.peekByte(0) == '_' {
			l.advance()
		}
	}
	if l.peekByte(0) == 'e' || l.peekByte(0) == 'E' {
		kind = tokenFloat
		l.advance()
		if l.peekByte(0) == '+' || l.peekByte(0) == '-' {
			l.advance()
		}
		for isDigit(l.peekByte(0)) || l.peekByte(0) == '_' {
			l.advance()
		}
	}
	raw := l.input[begin:l.offset]
	clean := strings.ReplaceAll(raw, "_", "")
	if kind == tokenInt {
		v, err := strconv.ParseInt(clean, 10, 64)
		if err != nil {
			l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "invalid integer literal", l.span(start), "")
			return token{kind: tokenInvalid, text: raw, span: l.span(start)}
		}
		return token{kind: kind, text: raw, value: v, span: l.span(start)}
	}
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		l.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, "invalid float literal", l.span(start), "")
		return token{kind: tokenInvalid, text: raw, span: l.span(start)}
	}
	return token{kind: kind, text: raw, value: v, span: l.span(start)}
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F'
}

func isIdentifierStart(ch byte) bool {
	return ch == '_' || ch == '-' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}
func isIdentifierContinue(ch byte) bool {
	return isIdentifierStart(ch) || isDigit(ch) || ch == '.' || ch == '/'
}
func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isNumberStart(ch, next byte) bool {
	return isDigit(ch) || (ch == '+' || ch == '-') && isDigit(next)
}
func isBoundary(ch byte) bool {
	return ch == 0 || ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == ';' || ch == '}' || ch == '{'
}
