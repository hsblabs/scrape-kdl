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
