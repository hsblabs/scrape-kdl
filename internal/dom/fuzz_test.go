package dom

import (
	"strings"
	"testing"
)

func FuzzParseSelectorNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"h1",
		"ul.items > li:nth-child(2n+1)",
		"a[href^='/horse/']:not(.disabled)",
		"div, span.value",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, selector string) {
		_, _ = ParseSelector(selector)
	})
}

func FuzzParseHTMLNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"<!doctype html><html><body><h1>Hello</h1></body></html>",
		"<table><tr><td>A<td>B</table>",
		"<div title='x'>&amp;<span>text</span></div>",
		"<script>const x = '<tag>';</script>",
		"<script>before</scriptx><b>inside</b></ScRiPt >",
		"<table><thead><tr><th>H<tbody><tr><td>A<td>B<tfoot><tr><td>F</table>",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = ParseHTML(strings.NewReader(source))
	})
}
