package source

import "testing"

func TestSpanString(t *testing.T) {
	span := Span{File: "example.kdl", Start: Position{Offset: 8, Line: 3, Column: 5}}
	if got := span.String(); got != "example.kdl:3:5" {
		t.Fatalf("Span.String() = %q", got)
	}
}

func TestMergeSpans(t *testing.T) {
	start := Span{
		File:  "example.kdl",
		Start: Position{Offset: 2, Line: 1, Column: 3},
		End:   Position{Offset: 4, Line: 1, Column: 5},
	}
	end := Span{
		File:  "example.kdl",
		Start: Position{Offset: 8, Line: 2, Column: 1},
		End:   Position{Offset: 12, Line: 2, Column: 5},
	}
	got := Merge(start, end)
	if got.File != start.File || got.Start != start.Start || got.End != end.End {
		t.Fatalf("Merge(start, end) = %#v", got)
	}
	if got := Merge(Span{}, end); got != end {
		t.Fatalf("Merge(empty, end) = %#v", got)
	}
	if got := Merge(start, Span{}); got != start {
		t.Fatalf("Merge(start, empty) = %#v", got)
	}
}
