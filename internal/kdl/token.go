package kdl

import "github.com/hsblabs/scrape-kdl/internal/source"

type tokenKind int

const (
	tokenInvalid tokenKind = iota
	tokenEOF
	tokenNewline
	tokenSemicolon
	tokenSlashdash
	tokenLBrace
	tokenRBrace
	tokenEquals
	tokenLParen
	tokenRParen
	tokenIdentifier
	tokenString
	tokenInt
	tokenFloat
	tokenBool
	tokenNull
)

type token struct {
	kind  tokenKind
	text  string
	value any
	span  source.Span
}
