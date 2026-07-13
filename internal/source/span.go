package source

import "fmt"

type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Span struct {
	File  string   `json:"file"`
	Start Position `json:"start"`
	End   Position `json:"end"`
}

func (s Span) String() string {
	return fmt.Sprintf("%s:%d:%d", s.File, s.Start.Line, s.Start.Column)
}

func Merge(a, b Span) Span {
	if a.File == "" {
		return b
	}
	if b.File == "" {
		return a
	}
	return Span{File: a.File, Start: a.Start, End: b.End}
}
