package dom

import (
	"strings"
	"testing"
)

func TestQueryAllPortableSelectors(t *testing.T) {
	document, err := ParseHTML(strings.NewReader(`<!doctype html><html><body>
<ul id="items" class="list"><li class="entry first" data-k="a"><a href="/a">A</a></li><li class="entry" data-k="b"><a href="/b">B</a></li><li class="entry"><span>C</span></li></ul>
</body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		selector string
		want     int
	}{
		{"#items > li.entry", 3},
		{"ul.list li:first-child", 1},
		{"li:nth-child(2)", 1},
		{"li:nth-last-child(1)", 1},
		{"li[data-k^='a']", 1},
		{"li.first + li", 1},
		{"li.first ~ li", 2},
		{"li:not(.first)", 2},
		{"li > a[href], li > span", 3},
	}
	for _, test := range tests {
		t.Run(test.selector, func(t *testing.T) {
			selector, err := ParseSelector(test.selector)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(QueryAll(document, selector)); got != test.want {
				t.Fatalf("got %d matches, want %d", got, test.want)
			}
		})
	}
}

func TestQueryLimitPreservesDocumentOrderAndStopsAtLimit(t *testing.T) {
	document, err := ParseHTML(strings.NewReader(`<main><i id="one"></i><i id="two"></i><i id="three"></i></main>`))
	if err != nil {
		t.Fatal(err)
	}
	selector, err := ParseSelector("i")
	if err != nil {
		t.Fatal(err)
	}

	limited := QueryLimit(document, selector, 2)
	if len(limited) != 2 {
		t.Fatalf("limited matches = %d, want 2", len(limited))
	}
	for index, want := range []string{"one", "two"} {
		if got, _ := limited[index].Attr("id"); got != want {
			t.Fatalf("limited[%d] id = %q, want %q", index, got, want)
		}
	}
	if got := len(QueryLimit(document, selector, 0)); got != 3 {
		t.Fatalf("unbounded matches = %d, want 3", got)
	}
}

func BenchmarkQueryAllLargeDocument(b *testing.B) {
	document, selector := benchmarkSelectorDocument(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if got := len(QueryAll(document, selector)); got != 10_000 {
			b.Fatalf("matches = %d", got)
		}
	}
}

func BenchmarkQueryLimitLargeDocument(b *testing.B) {
	document, selector := benchmarkSelectorDocument(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if got := len(QueryLimit(document, selector, 1)); got != 1 {
			b.Fatalf("matches = %d", got)
		}
	}
}

func benchmarkSelectorDocument(b *testing.B) (*Node, Selector) {
	b.Helper()
	var source strings.Builder
	source.WriteString("<main>")
	for range 10_000 {
		source.WriteString(`<i class="entry"></i>`)
	}
	source.WriteString("</main>")
	document, err := ParseHTML(strings.NewReader(source.String()))
	if err != nil {
		b.Fatal(err)
	}
	selector, err := ParseSelector(".entry")
	if err != nil {
		b.Fatal(err)
	}
	return document, selector
}

func TestQueryAllPortableSelectorOperatorsAndTypePseudos(t *testing.T) {
	document := mustParseHTML(t, `<main>
<div id="attrs" data-token="alpha beta" lang="en-US" data-value="prefix-middle-suffix"></div>
<div id="types"><span id="s1"></span><em id="e1"></em><span id="s2">text</span><span id="s3"></span></div>
<div id="only"><i id="i1"></i><b id="b1"></b></div>
<div id="single"><a id="a1"></a></div>
</main>`)
	tests := []struct {
		selector string
		wantIDs  []string
	}{
		{`[data-token="alpha beta"]`, []string{"attrs"}},
		{`[data-token~="beta"]`, []string{"attrs"}},
		{`[data-token~="alp"]`, nil},
		{`[lang|="en"]`, []string{"attrs"}},
		{`[lang|="en-US"]`, []string{"attrs"}},
		{`[lang|="e"]`, nil},
		{`[missing="value"]`, nil},
		{`[data-value^="prefix"]`, []string{"attrs"}},
		{`[data-value$="suffix"]`, []string{"attrs"}},
		{`[data-value*="middle"]`, []string{"attrs"}},
		{`[data-value*="absent"]`, nil},
		{`#types > span:empty`, []string{"s1", "s3"}},
		{`#types > span:last-child`, []string{"s3"}},
		{`#types > span:first-of-type`, []string{"s1"}},
		{`#types > span:last-of-type`, []string{"s3"}},
		{`#types > span:only-of-type`, nil},
		{`#only > i:only-of-type`, []string{"i1"}},
		{`#single > a:only-child`, []string{"a1"}},
		{`#types > span:nth-of-type(2)`, []string{"s2"}},
		{`#types > span:nth-last-of-type(2)`, []string{"s2"}},
		{`#types > span:nth-of-type(2n+1)`, []string{"s1", "s3"}},
		{`#types > span:nth-of-type(-n+2)`, []string{"s1", "s2"}},
	}
	for _, tt := range tests {
		t.Run(tt.selector, func(t *testing.T) {
			selector, err := ParseSelector(tt.selector)
			if err != nil {
				t.Fatal(err)
			}
			nodes := QueryAll(document, selector)
			gotIDs := make([]string, 0, len(nodes))
			for _, node := range nodes {
				id, _ := node.Attr("id")
				gotIDs = append(gotIDs, id)
			}
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("ids = %v, want %v", gotIDs, tt.wantIDs)
			}
			for index := range gotIDs {
				if gotIDs[index] != tt.wantIDs[index] {
					t.Fatalf("ids = %v, want %v", gotIDs, tt.wantIDs)
				}
			}
		})
	}

	selector, err := ParseSelector(`#s3, #types > span, #s1`)
	if err != nil {
		t.Fatal(err)
	}
	nodes := QueryAll(document, selector)
	gotIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		id, _ := node.Attr("id")
		gotIDs = append(gotIDs, id)
	}
	wantIDs := []string{"s1", "s2", "s3"}
	for index := range wantIDs {
		if len(gotIDs) != len(wantIDs) || gotIDs[index] != wantIDs[index] {
			t.Fatalf("selector list ids = %v, want %v", gotIDs, wantIDs)
		}
	}
}

func TestTextAndInnerHTML(t *testing.T) {
	document, err := ParseHTML(strings.NewReader(`<div id="x">A<br><span data-z="1">B</span></div>`))
	if err != nil {
		t.Fatal(err)
	}
	selector, _ := ParseSelector("#x")
	node := QueryAll(document, selector)[0]
	if got := node.TextContent(); got != "AB" {
		t.Fatalf("text = %q", got)
	}
	if got := node.InnerHTML(); got != `A<br><span data-z="1">B</span>` {
		t.Fatalf("html = %q", got)
	}
}

func TestParseSelectorClassifiesAttributeFlagsAsUnsupported(t *testing.T) {
	for _, selector := range []string{`[href="x" i]`, `[href=x s]`, `[href="x" I ]`} {
		_, err := ParseSelector(selector)
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("ParseSelector(%q) error = %v", selector, err)
		}
	}
	for _, selector := range []string{`[href="x"i]`, `[href="x" flag]`} {
		_, err := ParseSelector(selector)
		if err == nil || strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("ParseSelector(%q) error = %v", selector, err)
		}
	}
}
